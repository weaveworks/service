package main

import (
	"io"
	"net"
	"net/http"

	"github.com/Sirupsen/logrus"
)

func appProxy(a authenticator, m organizationMapper, w http.ResponseWriter, r *http.Request) {
	authResponse, err := a.authenticate(r)
	if err != nil {
		unauth, ok := err.(unauthorized)
		if ok {
			logrus.Infof("proxy: unauthorized request: %d", unauth.httpStatus)
			http.Error(w, unauth.Error(), unauth.httpStatus)
		} else {
			logrus.Errorf("proxy: error contacting authenticator: %v", err)
			w.WriteHeader(http.StatusBadGateway)
		}
		return
	}

	targetHost, err := m.getOrganizationsHost(authResponse.organizationID)
	if err != nil {
		logrus.Errorf("proxy: cannot get host for organization '%s': %v",
			authResponse.organizationID, err)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	logrus.Infof("proxy: mapping organization '%s' to host '%s'",
		authResponse.organizationID, targetHost)

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
	errChannel := make(chan error, 2)
	cp := func(dst io.Writer, src io.Reader) {
		_, err := io.Copy(dst, src)
		errChannel <- err
		logrus.Debugf("proxy: copier exited")
	}
	logrus.Debugf("proxy: spawning copiers")
	go cp(targetConn, clientConn)
	go cp(clientConn, targetConn)
	<-errChannel
	logrus.Debugf("proxy: connection closed")
}
