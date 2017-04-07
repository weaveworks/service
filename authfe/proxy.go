package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/opentracing-contrib/go-stdlib/nethttp"
	"github.com/opentracing/opentracing-go"
	"github.com/weaveworks/common/middleware"
)

const defaultPort = "80"

type proxy struct {
	// You should set this
	name string

	// Flag-set stuff
	hostAndPort string
	readOnly    bool

	// Internally used stuff
	sync.Once
	reverseProxy httputil.ReverseProxy
}

func (p *proxy) RegisterFlags(f *flag.FlagSet) {
	f.StringVar(&p.hostAndPort, p.name, "", fmt.Sprintf("Hostname & port for %s service", p.name))
	f.BoolVar(&p.readOnly, p.name+"-readonly", false, fmt.Sprintf("Make %s service, read-only (will only accept GETs)", p.name))
}

var readOnlyMethods = []string{
	http.MethodGet,
	http.MethodHead,
	http.MethodOptions,
}

var proxyTransport http.RoundTripper = &nethttp.Transport{
	RoundTripper: &http.Transport{
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
	},
}

func (p *proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.Once.Do(func() {
		// Make all transformations outside of the director since
		// they are also required when proxying websockets
		p.reverseProxy = httputil.ReverseProxy{
			Director:  func(*http.Request) {},
			Transport: proxyTransport,
		}
	})
	if !middleware.IsWSHandshakeRequest(r) {
		var ht *nethttp.Tracer
		r, ht = nethttp.TraceRequest(opentracing.GlobalTracer(), r)
		defer ht.Finish()
	}

	if p.hostAndPort == "" {
		w.WriteHeader(http.StatusNotImplemented)
		return
	}

	if p.readOnly {
		accepted := false
		for _, method := range readOnlyMethods {
			accepted = (method == r.Method)
			if accepted {
				break
			}
		}
		if !accepted {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
	}

	// Tweak request before sending
	r.Host = p.hostAndPort
	r.URL.Host = p.hostAndPort
	r.URL.Scheme = "http"

	log.Debugf("Forwarding %s %s to %s, final URL: %s", r.Method, r.RequestURI, p.hostAndPort, r.URL)

	// Detect whether we should do websockets
	if middleware.IsWSHandshakeRequest(r) {
		log.Debugf("proxy: detected websocket handshake")
		p.proxyWS(w, r)
		return
	}

	// Proxy request
	p.reverseProxy.ServeHTTP(w, r)
}

func (p *proxy) proxyWS(w http.ResponseWriter, r *http.Request) {
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
