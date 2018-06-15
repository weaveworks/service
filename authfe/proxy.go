package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"
	"sync"
	"time"

	"github.com/dlespiau/balance"

	"github.com/opentracing-contrib/go-stdlib/nethttp"
	"github.com/opentracing/opentracing-go"
	log "github.com/sirupsen/logrus"
	httpgrpc_server "github.com/weaveworks/common/httpgrpc/server"
	"github.com/weaveworks/common/logging"
	"github.com/weaveworks/common/middleware"
)

const defaultPort = "80"

func makeLoadBalancer(cfg proxyConfig) (balance.LoadBalancer, error) {
	var balancer balance.LoadBalancer
	needAffinitykey := true

	switch cfg.l7.algorithm {
	case "":
		// No load balancer configured.
		return nil, nil
	case "consistent":
		balancer = balance.NewConsistent(balance.ConsistentConfig{})
	case "bounded-load":
		balancer = balance.NewConsistent(balance.ConsistentConfig{
			LoadFactor: cfg.l7.loadFactor,
		})
	default:
		return nil, fmt.Errorf("unknown load balancing algorithm: %s", cfg.l7.algorithm)
	}

	if needAffinitykey && cfg.l7.affinityKey == "" {
		return nil, fmt.Errorf("an affinity load balancing scheme was configured, but no affinity key provided")
	}
	return balance.WithServiceFallback(balancer, cfg.hostAndPort)
}

func newProxy(cfg proxyConfig) (http.Handler, error) {
	if cfg.grpcHost != "" {
		return httpgrpc_server.NewClient(cfg.grpcHost)
	}
	switch cfg.protocol {
	case "grpc":
		return httpgrpc_server.NewClient(cfg.hostAndPort)
	case "https":
		fallthrough
	case "http":
		balancer, err := makeLoadBalancer(cfg)
		if err != nil {
			return nil, err
		}

		// Make all transformations outside of the director since
		// they are also required when proxying websockets
		return &httpProxy{
			cfg,
			balancer,
			httputil.ReverseProxy{
				Director:  func(*http.Request) {},
				Transport: proxyTransport(cfg),
			},
		}, nil
	case "mock":
		return &mockProxy{cfg}, nil
	}
	return nil, fmt.Errorf("Unknown protocol %q for service %s", cfg.protocol, cfg.name)
}

type httpProxy struct {
	proxyConfig
	balancer     balance.LoadBalancer
	reverseProxy httputil.ReverseProxy
}

var readOnlyMethods = map[string]struct{}{
	http.MethodGet:     {},
	http.MethodHead:    {},
	http.MethodOptions: {},
}

// We have two different Transports:
//   - One with Keep-alive disabled to ensure L4 load-balancing. With each new
//   connection a different service endpoint will be selected, at the expanse of a
//   bit more latency.
//   - One with Keep-alive enabled for use with L7 load-balancing.

var proxyTransportL7 http.RoundTripper = &nethttp.Transport{
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

var proxyTransportNoKeepAlive http.RoundTripper = &nethttp.Transport{
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

func proxyTransport(cfg proxyConfig) http.RoundTripper {
	if cfg.l7.algorithm == "" {
		return proxyTransportNoKeepAlive
	}
	return proxyTransportL7
}

func (p *httpProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !middleware.IsWSHandshakeRequest(r) {
		var ht *nethttp.Tracer
		cmo := nethttp.OperationName(fmt.Sprintf("%s %s", r.Method, r.URL.Path))
		r, ht = nethttp.TraceRequest(opentracing.GlobalTracer(), r, cmo)
		defer ht.Finish()
	}

	if p.hostAndPort == "" {
		w.WriteHeader(http.StatusNotImplemented)
		return
	}

	if p.readOnly {
		_, ok := readOnlyMethods[r.Method]
		if !ok {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
	}

	hostandPort := p.hostAndPort
	if p.balancer {
		key := r.Header.Get(p.cfg.l7.affinityKey)
		if key == "" {
			http.Error(w, fmt.Sprintf("no affinity header defined: %s", p.cfg.l7.affinity), http.StatusBadRequest)
			return
		}

		hostandPort = p.balancer.Get(key).Key()
	}

	// Tweak request before sending
	r.Header.Add("X-Forwarded-Host", r.Host) // Used for previews of UI builds at https://1234.build.dev.weave.works
	r.Host = hostandPort
	r.URL.Host = hostandPort
	r.URL.Scheme = p.protocol

	logging.With(r.Context()).Debugf("Forwarding %s %s to %s, final URL: %s", r.Method, r.RequestURI, p.hostAndPort, r.URL)

	// Detect whether we should do websockets
	if middleware.IsWSHandshakeRequest(r) {
		logging.With(r.Context()).Debugf("proxy: detected websocket handshake")
		p.proxyWS(w, r)
		return
	}

	// Proxy request
	p.reverseProxy.ServeHTTP(w, r)

	if p.balancer {
		p.balancer.Put(hostandPort)
	}
}

func (p *httpProxy) proxyWS(w http.ResponseWriter, r *http.Request) {
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
		logging.With(r.Context()).Errorf("proxy: websocket: error dialing backend %q: %v", p.hostAndPort, err)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	defer targetConn.Close()

	// Hijack the connection to copy raw data back to our client
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		logging.With(r.Context()).Errorf("proxy: websocket: error casting to Hijacker on request to %q", p.hostAndPort)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		logging.With(r.Context()).Errorf("proxy: websocket: Hijack error: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer clientConn.Close()

	// Forward current request to the target host since it was received before hijacking
	logging.With(r.Context()).Debugf("proxy: websocket: writing original request to %s%s", p.hostAndPort, r.URL.Opaque)
	if err := r.Write(targetConn); err != nil {
		logging.With(r.Context()).Errorf("proxy: websocket: error copying request to target: %v", err)
		return
	}

	// Copy websocket payload back and forth between our client and the target host
	var wg sync.WaitGroup
	wg.Add(2)
	logging.With(r.Context()).Debugf("proxy: websocket: spawning copiers")
	go copyStream(clientConn, targetConn, &wg, "proxy: websocket: \"server2client\"")
	go copyStream(targetConn, clientConn, &wg, "proxy: websocket: \"client2server\"")
	wg.Wait()
	logging.With(r.Context()).Debugf("proxy: websocket: connection closed")
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

// mockProxy wrties the proxy name in the body for testing
type mockProxy struct {
	proxyConfig
}

func (p *mockProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, p.name)
}
