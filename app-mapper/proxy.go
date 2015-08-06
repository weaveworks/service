package main

import (
	"io"
	"net"
	"net/http"
	"regexp"
	"sync"

	"github.com/Sirupsen/logrus"
)

var prefix = regexp.MustCompile("^/api/app/[^/]+")

func appProxy(a authenticator, m organizationMapper, w http.ResponseWriter, r *http.Request, org string) {
	authResponse, err := a.authenticate(r, org)
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

	targetHost, err := m.getOrganizationsHost(authResponse.OrganizationID)
	if err != nil {
		logrus.Errorf(
			"proxy: cannot get host for organization %q: %v",
			authResponse.OrganizationID,
			err,
		)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	logrus.Infof(
		"proxy: mapping organization %q to host %q",
		authResponse.OrganizationID,
		targetHost,
	)

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

	// Forward current request to the target host since it was received before hijacking
	logrus.Debugf("proxy: writing original request to %q", targetHost)

	// Ensure the URL is incorrecly munged by go.
	r.URL.Opaque = r.RequestURI
	r.URL.RawQuery = ""

	// Trim /api/app/<foo> off the front of URL
	r.URL.Opaque = prefix.ReplaceAllLiteralString(r.URL.Opaque, "")

	if err = r.Write(targetConn); err != nil {
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

func addPort(host, defaultPort string) string {
	_, _, err := net.SplitHostPort(host)
	if err == nil {
		// it had a port number
		return host
	}
	return net.JoinHostPort(host, defaultPort)
}
