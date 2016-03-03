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
	scope "github.com/weaveworks/scope/xfer"
)

var appPrefix = regexp.MustCompile("^/api/app/[^/]+")

type proxy struct {
	authenticator authenticator
	mapper        organizationMapper
	probeBumper   probeBumper
	reverseProxy  httputil.ReverseProxy
}

func newProxy(a authenticator, m organizationMapper, b probeBumper) proxy {
	// Make all transformations outside of the director since
	// they are also required when proxying websockets
	emptyDirector := func(*http.Request) {}
	return proxy{
		authenticator: a,
		mapper:        m,
		probeBumper:   b,
		reverseProxy:  httputil.ReverseProxy{Director: emptyDirector},
	}
}

func (p proxy) registerHandlers(router *mux.Router) {
	// Route names are used by instrumentation.
	router.Path("/api/app/{orgName}").Name("api_app_redirect").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		orgName := mux.Vars(r)["orgName"]
		http.Redirect(w, r, fmt.Sprintf("/api/app/%s/", orgName), http.StatusMovedPermanently)
	})

	fwd := authOrgHandler(p.authenticator,
		func(r *http.Request) string { return mux.Vars(r)["orgName"] },
		func(w http.ResponseWriter, r *http.Request, orgID string) {
			// Trim /api/app/<orgName> off the front of the URI
			r.RequestURI = appPrefix.ReplaceAllLiteralString(r.RequestURI, "")
			p.forwardRequest(w, r, orgID)
		},
	)
	// We believe forwards to /api/topology/{topology}/ws will normally have
	// very high latencies. We separate that endpoint from the general
	// PathPrefix in order to give it a special name, which we exclude from
	// the latency alerting rules in our monitoring. See service #291.
	router.Path("/api/app/{orgName}/api/topology/{topology}/ws").Name("api_app_topology_ws").Handler(fwd)
	router.PathPrefix("/api/app/{orgName}/").Name("api_app").Handler(fwd)

	router.Path("/api/control/ws").Name("api_control").Handler(authProbeHandler(p.authenticator,
		func(w http.ResponseWriter, r *http.Request, orgID string) {
			p.forwardRequest(w, r, orgID)
		},
	))

	// Pipe deletion from probe
	router.Path("/api/pipe/{pipeID}").Methods("DELETE").Name("api_pipe_probe_delete").Handler(authProbeHandler(p.authenticator,
		func(w http.ResponseWriter, r *http.Request, orgID string) {
			p.forwardRequest(w, r, orgID)
		},
	))

	// Pipe websocket from probe
	router.Path("/api/pipe/{pipeID}/probe").Name("api_pipe_probe").Handler(authProbeHandler(p.authenticator,
		func(w http.ResponseWriter, r *http.Request, orgID string) {
			p.forwardRequest(w, r, orgID)
		},
	))

	router.Path("/api/report").Name("api_report").Handler(authProbeHandler(p.authenticator,
		func(w http.ResponseWriter, r *http.Request, orgID string) {
			if probeID := r.Header.Get(scope.ScopeProbeIDHeader); probeID == "" {
				logrus.Error("proxy: probe with missing identification header")
			} else {
				if err := p.probeBumper.bumpProbeLastSeen(probeID, orgID); err != nil {
					logrus.Warnf("proxy: cannot bump probe's last-seen (%q, %q): %v", probeID, orgID, err)
				}
			}
			p.forwardRequest(w, r, orgID)
		},
	))
}

func (p proxy) forwardRequest(w http.ResponseWriter, r *http.Request, orgID string) {
	hostInfo, err := p.mapper.getOrganizationsHost(orgID)
	if err != nil {
		logrus.Errorf("proxy: cannot get host for organization with ID %q: %v", orgID, err)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	if !hostInfo.IsReady {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	targetHostPort := addPort(hostInfo.HostName, scope.AppPort)

	// Tweak request before sending
	r.Host = targetHostPort
	r.URL.Host = targetHostPort
	r.URL.Scheme = "http"

	// Ensure the URL is not incorrectly munged by go.
	r.URL.Opaque = r.RequestURI
	r.URL.RawQuery = ""

	logrus.Debugf("proxy: forwardRequest: forwarding %s %s%s to orgID %q", r.Method, targetHostPort, r.URL.Opaque, orgID)

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
	wsRequestCount.Inc()
	wsConnections.Inc()
	defer wsConnections.Dec()

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
		logrus.Errorf("proxy: websocket: error casting to Hijacker on request to %q", targetHost)
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
	logrus.Debugf("proxy: websocket: writing original request to %s%s", targetHost, r.URL.Opaque)
	if err := r.Write(targetConn); err != nil {
		logrus.Errorf("proxy: websocket: error copying request to target: %v", err)
		return
	}

	// Copy websocket payload back and forth between our client and the target host
	var wg sync.WaitGroup
	wg.Add(2)
	logrus.Debugf("proxy: websocket: spawning copiers")
	go copyStream(clientConn, targetConn, &wg, "proxy: websocket: \"server2client\"")
	go copyStream(targetConn, clientConn, &wg, "proxy: websocket: \"client2server\"")
	wg.Wait()
	logrus.Debugf("proxy: websocket: connection closed")
}

type closeWriter interface {
	CloseWrite() error
}

func copyStream(dst io.WriteCloser, src io.Reader, wg *sync.WaitGroup, tag string) {
	defer wg.Done()
	if _, err := io.Copy(dst, src); err != nil {
		logrus.Warnf("%s: io.Copy: %s", tag, err)
	}
	var err error
	if c, ok := dst.(closeWriter); ok {
		err = c.CloseWrite()
	} else {
		err = dst.Close()
	}
	if err != nil {
		logrus.Warningf("%s: error closing connection: %s", tag, err)
	}
	logrus.Debugf("%s: copier exited", tag)
}

func addPort(host string, defaultPort int) string {
	_, _, err := net.SplitHostPort(host)
	if err == nil {
		// it had a port number
		return host
	}
	return fmt.Sprintf("%s:%d", host, defaultPort)
}
