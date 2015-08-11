package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"regexp"
	"strings"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
)

const scopeDefaultPortNumber = 4040

var prefix = regexp.MustCompile("^/api/app/[^/]+")

type proxy struct {
	authenticator authenticator
	mapper        organizationMapper
	reverseProxy  httputil.ReverseProxy
}

func newProxy(a authenticator, m organizationMapper) proxy {
	// Make all transformations outside of the director since
	// they are also required when proxying websockets
	emptyDirector := func(*http.Request) {}
	return proxy{
		authenticator: a,
		mapper:        m,
		reverseProxy:  httputil.ReverseProxy{Director: emptyDirector},
	}
}

func (p proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	router := mux.NewRouter()
	router.Path("/api/app/{orgName}").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		orgName := mux.Vars(r)["orgName"]
		w.Header().Set("Location", fmt.Sprintf("/api/app/%s/", orgName))
		w.WriteHeader(301)
	})
	router.PathPrefix("/api/app/{orgName}/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		orgName := mux.Vars(r)["orgName"]
		p.run(w, r, orgName)
	})
	router.ServeHTTP(w, r)
}

func (p proxy) run(w http.ResponseWriter, r *http.Request, orgName string) {
	authResponse, err := p.authenticator.authenticate(r, orgName)
	if err != nil {
		if unauth, ok := err.(unauthorized); ok {
			logrus.Infof("proxy: unauthorized request: %d", unauth.httpStatus)
			w.WriteHeader(http.StatusUnauthorized)
		} else {
			logrus.Errorf("proxy: error contacting authenticator: %v", err)
			w.WriteHeader(http.StatusBadGateway)
		}
		return
	}

	orgID := authResponse.OrganizationID
	targetHost, err := p.mapper.getOrganizationsHost(orgID)
	if err != nil {
		logrus.Errorf("proxy: cannot get host for organization with ID %q: %v", orgID, err)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	targetHostPort := addPort(targetHost, scopeDefaultPortNumber)
	logrus.Infof("proxy: mapping organization with ID %q to host %q", orgID, targetHostPort)

	// Tweak request before sending
	r.Host = targetHostPort
	r.URL.Host = targetHostPort
	r.URL.Scheme = "http"

	// Ensure the URL is not incorrectly munged by go.
	r.URL.Opaque = r.RequestURI
	r.URL.RawQuery = ""

	// Trim /api/app/<orgName> off the front of URL
	r.URL.Opaque = prefix.ReplaceAllLiteralString(r.URL.Opaque, "")

	// Detect whether we should do websockets
	if isWSHandshakeRequest(r) {
		logrus.Debugf("proxy: detected websocket handshake")
		proxyWS(targetHostPort, w, r)
		return
	}

	// Proxy request
	p.reverseProxy.ServeHTTP(w, r)
}

func isWSHandshakeRequest(req *http.Request) bool {
	return strings.ToLower(req.Header.Get("Upgrade")) == "websocket" &&
		strings.ToLower(req.Header.Get("Connection")) == "upgrade"
}

func proxyWS(targetHost string, w http.ResponseWriter, r *http.Request) {
	// Connect to target
	targetConn, err := net.Dial("tcp", targetHost)
	if err != nil {
		logrus.Errorf("proxy: websocket: error dialing backend %q: %v", targetHost, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer targetConn.Close()

	// Hijack the connection to copy raw data back to our client
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		logrus.Errorf("proxy: websocket: error casting to Hijacker on %q: %v", targetHost, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		logrus.Errorf("proxy: websocket: Hijack error: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer clientConn.Close()

	// Forward current request to the target host since it was received before hijacking
	logrus.Debugf("proxy: websocket: writing original request to http://%s%s", targetHost, r.URL.Opaque)
	if err := r.Write(targetConn); err != nil {
		logrus.Errorf("proxy: websocket: error copying request to target: %v", err)
		return
	}

	// Copy websocket payload back and forth between our client and the target host
	var wg sync.WaitGroup
	cp := func(dst io.Writer, src io.Reader, tag string) {
		defer wg.Done()
		io.Copy(dst, src)
		logrus.Debugf("proxy: websocket: %q copier exited", tag)
	}
	wg.Add(2)
	logrus.Debugf("proxy: websocket: spawning copiers")
	go cp(clientConn, targetConn, "server2client")
	go cp(targetConn, clientConn, "client2server")
	wg.Wait()
	logrus.Debugf("proxy: websocket: connection closed")
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

func addPort(host string, defaultPort int) string {
	_, _, err := net.SplitHostPort(host)
	if err == nil {
		// it had a port number
		return host
	}
	return fmt.Sprintf("%s:%d", host, defaultPort)
}
