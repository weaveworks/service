package main

import (
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
)

const defaultPort = "80"

type proxy struct {
	hostAndPort  string
	reverseProxy httputil.ReverseProxy
}

var proxyTransport http.RoundTripper = &http.Transport{
	// No connection pooling, increases latency, but ensures fair load-balancing.
	DisableKeepAlives: true,

	// Rest are from http.DefaultTransport
	Proxy: http.ProxyFromEnvironment,
	DialContext: (&net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext,
	TLSHandshakeTimeout:   10 * time.Second,
	ExpectContinueTimeout: 1 * time.Second,
}

func newProxy(hostAndPort string) proxy {
	// Make all transformations outside of the director since
	// they are also required when proxying websockets
	emptyDirector := func(*http.Request) {}
	return proxy{
		hostAndPort: hostAndPort,
		reverseProxy: httputil.ReverseProxy{
			Director:  emptyDirector,
			Transport: proxyTransport,
		},
	}
}

func (p proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if p.hostAndPort == "" {
		w.WriteHeader(http.StatusNotImplemented)
		return
	}

	// Tweak request before sending
	r.Host = p.hostAndPort
	r.URL.Host = p.hostAndPort
	r.URL.Scheme = "http"

	// Ensure the URL is not incorrectly munged by go.
	r.URL.Opaque = r.RequestURI
	r.URL.RawQuery = ""

	log.Debugf("Forwarding %s %s to %s", r.Method, r.RequestURI, p.hostAndPort)

	// Detect whether we should do websockets
	if isWSHandshakeRequest(r) {
		log.Debugf("proxy: detected websocket handshake")
		p.proxyWS(w, r)
		return
	}

	// Proxy request
	p.reverseProxy.ServeHTTP(w, r)
}

func isWSHandshakeRequest(req *http.Request) bool {
	if strings.ToLower(req.Header.Get("Upgrade")) == "websocket" {
		// Connection header values can be of form "foo, bar, ..."
		parts := strings.Split(strings.ToLower(req.Header.Get("Connection")), ",")
		for _, part := range parts {
			if strings.TrimSpace(part) == "upgrade" {
				return true
			}
		}
	}
	return false
}

func (p proxy) proxyWS(w http.ResponseWriter, r *http.Request) {
	wsRequestCount.Inc()
	wsConnections.Inc()
	defer wsConnections.Dec()

	address := p.hostAndPort
	if !strings.Contains(address, ":") {
		address = address + ":" + defaultPort
	}

	// Connect to target
	targetConn, err := net.Dial("tcp", address)
	if err != nil {
		log.Errorf("proxy: websocket: error dialing backend %q: %v", p.hostAndPort, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer targetConn.Close()

	// Hijack the connection to copy raw data back to our client
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		log.Errorf("proxy: websocket: error casting to Hijacker on request to %q", p.hostAndPort)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		log.Errorf("proxy: websocket: Hijack error: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer clientConn.Close()

	// Forward current request to the target host since it was received before hijacking
	log.Debugf("proxy: websocket: writing original request to %s%s", p.hostAndPort, r.URL.Opaque)
	if err := r.Write(targetConn); err != nil {
		log.Errorf("proxy: websocket: error copying request to target: %v", err)
		return
	}

	// Copy websocket payload back and forth between our client and the target host
	var wg sync.WaitGroup
	wg.Add(2)
	log.Debugf("proxy: websocket: spawning copiers")
	go copyStream(clientConn, targetConn, &wg, "proxy: websocket: \"server2client\"")
	go copyStream(targetConn, clientConn, &wg, "proxy: websocket: \"client2server\"")
	wg.Wait()
	log.Debugf("proxy: websocket: connection closed")
}

type closeWriter interface {
	CloseWrite() error
}

func copyStream(dst io.WriteCloser, src io.Reader, wg *sync.WaitGroup, tag string) {
	defer wg.Done()
	if _, err := io.Copy(dst, src); err != nil {
		log.Warnf("%s: io.Copy: %s", tag, err)
	}
	var err error
	if c, ok := dst.(closeWriter); ok {
		err = c.CloseWrite()
	} else {
		err = dst.Close()
	}
	if err != nil {
		log.Warningf("%s: error closing connection: %s", tag, err)
	}
	log.Debugf("%s: copier exited", tag)
}
