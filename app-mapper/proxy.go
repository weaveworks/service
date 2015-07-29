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
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	if authResponse.httpStatus != http.StatusOK {
		w.WriteHeader(authResponse.httpStatus)
		return
	}

	targetHost, err := m.getOrganizationsHost(authResponse.organizationID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	logrus.Infof("proxy: mapping to %s", targetHost)

	targetConn, err := net.Dial("tcp", targetHost)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		logrus.Errorf("proxy: error dialing backend %s: %v", targetHost, err)
		return
	}
	defer targetConn.Close()

	// Hijack the connection to copy raw data back to the our client
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		w.WriteHeader(http.StatusInternalServerError)
		logrus.Errorf("proxy: error casting to Hijacker on %s: %v", targetHost, err)
		return
	}
	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		logrus.Errorf("proxy: Hijack error: %v", err)
		return
	}
	defer clientConn.Close()

	// Forward current request to the target host since it was received before
	// hijacking
	logrus.Debugf("proxy: writing original request to %s", targetHost)
	if err = r.Write(targetConn); err != nil {
		logrus.Errorf("proxy: error copying request to target: %v", err)
		return
	}

	// Copy information back and forth between our client and the target
	// host
	errChannel := make(chan error, 2)
	cp := func(dst io.Writer, src io.Reader) {
		_, err := io.Copy(dst, src)
		errChannel <- err
	}
	go cp(targetConn, clientConn)
	go cp(clientConn, targetConn)
	<-errChannel
	logrus.Debugf("proxy: connection closed")
}
