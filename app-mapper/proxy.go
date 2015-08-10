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
	"golang.org/x/net/websocket"
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
		strings.ToLower(req.Header.Get("Connection")) == "upgrade" &&
		req.Header.Get("Origin") != ""
}

func proxyWS(targetHost string, w http.ResponseWriter, r *http.Request) {
	// Connect to target
	url := "ws://" + targetHost + r.URL.Opaque
	wsTargetConfig, err := websocket.NewConfig(url, r.Header.Get("Origin"))
	if err != nil {
		logrus.Errorf("proxy: error creating websocket config: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	copyHeader(wsTargetConfig.Header, r.Header)
	wsTargetConn, err := websocket.DialConfig(wsTargetConfig)
	if err != nil {
		logrus.Errorf("proxy: error dialing %q: %v", url, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer wsTargetConn.Close()

	// Copy websocket payload back and forth between our client and the target host
	var wg sync.WaitGroup
	cp := func(dst *websocket.Conn, src *websocket.Conn, tag string) {
		defer wg.Done()
		io.Copy(dst, src)
		// Need to close the destination explicitly or the sibling go routine
		// will hang
		dst.Close()
		logrus.Debugf("proxy: websocket: %q copier exited", tag)
	}
	wsHandler := websocket.Handler(func(wsClientConn *websocket.Conn) {
		wg.Add(2)
		logrus.Debugf("proxy: websocket: spawning copiers")
		go cp(wsClientConn, wsTargetConn, "server2client")
		go cp(wsTargetConn, wsClientConn, "client2server")
		wg.Wait()
		logrus.Debugf("proxy: websocket: connection closed")
	})
	wsHandler.ServeHTTP(w, r)
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
