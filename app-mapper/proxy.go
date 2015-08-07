package main

import (
	"io"
	"net"
	"net/http"
	"regexp"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
)

var prefix = regexp.MustCompile("^/api/app/[^/]+")

func makeProxyHandler(a authenticator, m organizationMapper) http.Handler {
	proxyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		orgID := mux.Vars(r)["orgID"]
		appProxy(a, m, w, r, orgID)
	})
	router := mux.NewRouter()
	router.PathPrefix("/api/app/{orgID}/").Handler(proxyHandler)
	return router
}

func appProxy(a authenticator, m organizationMapper, w http.ResponseWriter, r *http.Request, orgID string) {
	if ok := confirmOrganization(a, w, r, orgID); !ok {
		return
	}

	targetHost := getTargetHost(m, w, r, orgID)
	if targetHost == "" {
		return
	}

	proxyFunc := func(clientConn net.Conn, targetConn net.Conn) {
		// Ensure the URL is not incorrectly munged by go.
		r.URL.Opaque = r.RequestURI
		r.URL.RawQuery = ""

		// Trim /api/app/<orgID> off the front of URL
		r.URL.Opaque = prefix.ReplaceAllLiteralString(r.URL.Opaque, "")

		// Forward current request to the target host since it was received before hijacking
		logrus.Debugf("proxy: writing original request to %q", targetHost)

		if err := r.Write(targetConn); err != nil {
			logrus.Errorf("proxy: error copying request to target: %v", err)
			return
		}

		// Copy information back and forth between our client and the target host
		var wg sync.WaitGroup
		cp := func(dst io.Writer, src io.Reader) {
			defer wg.Done()
			io.Copy(dst, src)
			logrus.Debugf("proxy: copier exited")
		}
		logrus.Debugf("proxy: spawning copiers")
		wg.Add(2)
		go cp(targetConn, clientConn)
		go cp(clientConn, targetConn)
		wg.Wait()
		logrus.Debugf("proxy: connection closed")
	}

	runProxy(targetHost, w, proxyFunc)
}

func confirmOrganization(a authenticator, w http.ResponseWriter, r *http.Request, orgID string) bool {
	authResponse, err := a.authenticate(r, orgID)
	if err != nil {
		if unauth, ok := err.(unauthorized); ok {
			logrus.Infof("proxy: unauthorized request: %d", unauth.httpStatus)
			w.WriteHeader(http.StatusUnauthorized)
		} else {
			logrus.Errorf("proxy: error contacting authenticator: %v", err)
			w.WriteHeader(http.StatusBadGateway)
		}
		return false
	}
	return authResponse.OrganizationID == orgID
}

func getTargetHost(m organizationMapper, w http.ResponseWriter, r *http.Request, orgID string) string {
	targetHost, err := m.getOrganizationsHost(orgID)
	if err != nil {
		logrus.Errorf("proxy: cannot get host for organization %q: %v", orgID, err)
		w.WriteHeader(http.StatusBadGateway)
		return ""
	}
	logrus.Infof("proxy: mapping organization %q to host %q", orgID, targetHost)
	return targetHost
}

func runProxy(targetHost string, w http.ResponseWriter, proxyFunc func(clientConn net.Conn, targetConn net.Conn)) {
	targetHostPort := addPort(targetHost, "80")
	targetConn, err := net.Dial("tcp", targetHostPort)

	if err != nil {
		logrus.Errorf("proxy: error dialing backend %q: %v", targetHost, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer targetConn.Close()

	// Hijack the connection to copy raw data back to our client
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		logrus.Errorf("proxy: error casting to Hijacker on %q: %v", targetHost, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		logrus.Errorf("proxy: Hijack error: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer clientConn.Close()

	proxyFunc(clientConn, targetConn)
}

func addPort(host, defaultPort string) string {
	_, _, err := net.SplitHostPort(host)
	if err == nil {
		// it had a port number
		return host
	}
	return net.JoinHostPort(host, defaultPort)
}
