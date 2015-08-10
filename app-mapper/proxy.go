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

var prefix = regexp.MustCompile("^/api/app/[^/]+")

type proxy struct {
	authenticator authenticator
	mapper        organizationMapper
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
	orgID := verifyOrganization(p.authenticator, w, r, orgName)
	if orgID == "" {
		return
	}

	targetHost := getTargetHost(p.mapper, w, r, orgID)
	if targetHost == "" {
		return
	}

	// Tweak request before sending
	r.Host = targetHost
	r.URL.Host = targetHost
	r.URL.Scheme = "http"

	// Ensure the URL is not incorrectly munged by go.
	r.URL.Opaque = r.RequestURI
	r.URL.RawQuery = ""

	// Trim /api/app/<orgName> off the front of URL
	r.URL.Opaque = prefix.ReplaceAllLiteralString(r.URL.Opaque, "")

	// Detect whether we should do websockets
	if isWSHandshakeRequest(r) {
		logrus.Debugf("proxy: detected websocket handshake")
		proxyWS(targetHost, w, r)
		return
	}

	// Send request
	res, err := http.DefaultTransport.RoundTrip(r)
	if err != nil {
		logrus.Errorf("proxy: error copying request to target: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer res.Body.Close()

	// Return response
	err = copyResponse(w, res)
	if err != nil {
		logrus.Errorf("proxy: error copying response back to client: %v", err)
	}
}

func verifyOrganization(a authenticator, w http.ResponseWriter, r *http.Request, orgName string) string {
	authResponse, err := a.authenticate(r, orgName)
	if err != nil {
		if unauth, ok := err.(unauthorized); ok {
			logrus.Infof("proxy: unauthorized request: %d", unauth.httpStatus)
			w.WriteHeader(http.StatusUnauthorized)
		} else {
			logrus.Errorf("proxy: error contacting authenticator: %v", err)
			w.WriteHeader(http.StatusBadGateway)
		}
		return ""
	}
	return authResponse.OrganizationID
}

func getTargetHost(m organizationMapper, w http.ResponseWriter, r *http.Request, orgID string) string {
	targetHost, err := m.getOrganizationsHost(orgID)
	if err != nil {
		logrus.Errorf("proxy: cannot get host for organization with ID %q: %v", orgID, err)
		w.WriteHeader(http.StatusBadGateway)
		return ""
	}
	logrus.Infof("proxy: mapping organization with ID %q to host %q", orgID, targetHost)
	return targetHost
}

func isWSHandshakeRequest(req *http.Request) bool {
	return strings.ToLower(req.Header.Get("Upgrade")) == "websocket" &&
		strings.ToLower(req.Header.Get("Connection")) == "upgrade"

}

func isWSHandshakeResponse(res *http.Response) bool {
	return res.StatusCode == http.StatusSwitchingProtocols &&
		strings.ToLower(res.Header.Get("Upgrade")) == "websocket" &&
		strings.ToLower(res.Header.Get("Connection")) == "upgrade"
}

func proxyWS(targetHost string, w http.ResponseWriter, wsHandshakeReq *http.Request) {
	// Use httputil's ClientConn to contact the target since we need to
	// reuse the connection after upgrading to websocket and http.Client's
	// connection is not hijackable
	targetHostPort := addPort(targetHost, "4040")
	targetConn, err := net.Dial("tcp", targetHostPort)
	if err != nil {
		logrus.Errorf("proxy: error dialing backend %q: %v", targetHost, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer targetConn.Close()
	client := httputil.NewClientConn(targetConn, nil)

	// Make sure the handshake goes through, otherwise stay in http
	logrus.Debugf("proxy: sending websocket handshake to target")
	wsHandshakeRes, err := client.Do(wsHandshakeReq)
	if err != nil {
		logrus.Errorf("proxy: error sending websocket handshake request to target %q: %v", targetHost, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Write the handshake response back, whatever that is
	logrus.Debugf("proxy: copying websocket response to client")
	err = copyResponse(w, wsHandshakeRes)
	if err != nil {
		logrus.Errorf("proxy: error copying websocket handshake response: %v", err)
		return
	}

	if !isWSHandshakeResponse(wsHandshakeRes) {
		// stay in http
		logrus.Infof("proxy: unexpected websocket handshake response: %v", *wsHandshakeRes)
		return
	}
	logrus.Debugf("proxy: websocket handshake finished")

	// Now we have upgraded to websocket

	// Go back to raw tcp with our target
	targetConn, _ = client.Hijack()
	if err != nil {
		logrus.Errorf("proxy: error hijacking client: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
	}

	// Hijack the connection to copy raw data back to our client
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		logrus.Errorf("proxy: error casting to Hijacker on %q: %v", targetHost, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	clientConn, buf, err := hijacker.Hijack()
	if err != nil {
		logrus.Errorf("proxy: Hijack error: %v", err)
		return
	}
	defer clientConn.Close()
	if err = buf.Flush(); err != nil {
		logrus.Errorf("proxy: cannot flush webserver buffer after Hijack: %v", err)
		return
	}

	// Copy websocket back and forth between our client and the target host
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

func copyResponse(w http.ResponseWriter, r *http.Response) error {
	for k, vv := range r.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(r.StatusCode)
	_, err := io.Copy(w, r.Body)
	return err
}
