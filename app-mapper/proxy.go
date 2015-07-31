package main

import (
	"io"
	"net"
	"net/http"
	"sync"

	"github.com/Sirupsen/logrus"
)

func appProxy(a authenticator, m organizationMapper, w http.ResponseWriter, r *http.Request) {
	authResponse, err := a.authenticate(r)
	if err != nil {
		unauth, ok := err.(unauthorized)
		if ok {
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
		logrus.Errorf("proxy: cannot get host for organization '%s': %v",
			authResponse.OrganizationID, err)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	logrus.Infof("proxy: mapping organization '%s' to host '%s'",
		authResponse.OrganizationID, targetHost)

	targetConn, err := net.Dial("tcp", targetHost)
	if err != nil {
		logrus.Errorf("proxy: error dialing backend %s: %v",
			targetHost, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer targetConn.Close()

	// Hijack the connection to copy raw data back to our client
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		logrus.Errorf("proxy: error casting to Hijacker on %s: %v",
			targetHost, err)
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
	logrus.Debugf("proxy: writing original request to '%s'", targetHost)
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
