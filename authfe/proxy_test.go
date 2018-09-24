package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	gorillaws "github.com/gorilla/websocket"
	"github.com/opentracing/opentracing-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	jaegercfg "github.com/uber/jaeger-client-go/config"
	"github.com/weaveworks/common/httpgrpc"
	httpgrpc_server "github.com/weaveworks/common/httpgrpc/server"
	"github.com/weaveworks/common/middleware"
	"github.com/weaveworks/common/user"
	"golang.org/x/net/websocket"
	"google.golang.org/grpc"
)

func TestProxyWebSocket(t *testing.T) {
	serverExited := uint32(0)

	// Use a websocket echo server in the target
	wsServer := httptest.NewServer(websocket.Handler(func(ws *websocket.Conn) {
		_, err := io.Copy(ws, ws)
		assert.NoError(t, err, "Target handler copy failed")
		atomic.StoreUint32(&serverExited, 1)
	}))
	defer wsServer.Close()

	// Setup a proxy server pointing at the websocket server
	wsURL, err := url.Parse(wsServer.URL)
	assert.NoError(t, err, "Cannot parse URL")
	proxy, _ := newProxy(proxyConfig{hostAndPort: wsURL.Host, protocol: "http"})
	proxyServer := httptest.NewServer(proxy)
	defer proxyServer.Close()

	// Establish a websocket connection with the proxy
	// Use the gorilla websocket library since its Conn.Close()
	// function doesn't send the Websocket close message (not giving
	// the server an opportunity to close the connection and forcing
	// the Proxy to explicitly do it).
	// This serves as a regression test for "App-mapper proxy leaks file descriptors" https://github.com/weaveworks/service/issues/253
	proxyURL, err := url.Parse(proxyServer.URL)
	assert.NoError(t, err, "Cannot parse URL")
	ws, _, err := gorillaws.DefaultDialer.Dial(
		fmt.Sprintf("ws://%s/foo/bar", proxyURL.Host),
		http.Header{"Origin": {"http://example.com"}},
	)
	assert.NoError(t, err, "Cannot dial WS")

	// We should receive back exactly what we send
	messageToSend := []byte("This is a test message")
	for i := 0; i < 100; i++ {
		err = ws.WriteMessage(gorillaws.TextMessage, messageToSend)
		assert.NoError(t, err, "Error sending message")

		_, messageToReceive, err := ws.ReadMessage()
		assert.NoError(t, err, "Error receiving message")
		assert.Equal(t, messageToSend, messageToReceive, "Unexpected echoed message")
	}

	ws.Close()
	time.Sleep(100 * time.Millisecond)
	assert.True(t, atomic.LoadUint32(&serverExited) == 1, "Server didn't exit")
}

// This tests ensure the proxy passes paths through without modifying it.
func TestProxyGet(t *testing.T) {
	expectedURI := "/foo//bar%2Fbaz?123"

	// Use a super simple server as the target
	handlerCalled := uint32(0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, r.RequestURI, expectedURI)
		atomic.StoreUint32(&handlerCalled, 1)
	}))
	defer server.Close()

	// Setup a proxy server pointing at the server
	serverURL, err := url.Parse(server.URL)
	assert.NoError(t, err, "Cannot parse URL")
	proxy, _ := newProxy(proxyConfig{hostAndPort: serverURL.Host, protocol: "http"})
	proxyServer := httptest.NewServer(proxy)
	defer proxyServer.Close()

	_, err = http.Get(fmt.Sprintf("%s%s", proxyServer.URL, expectedURI))
	assert.NoError(t, err, "Failed to get URL")
	assert.True(t, atomic.LoadUint32(&handlerCalled) == 1, "Server wasn't called")
}

func TestProxyReadOnly(t *testing.T) {
	expectedURI := "/foo"

	// Use a super simple server as the target
	handlerCalled := uint32(0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, r.RequestURI, expectedURI)
		atomic.StoreUint32(&handlerCalled, 1)
	}))
	defer server.Close()

	// Setup a proxy server pointing at the server
	serverURL, err := url.Parse(server.URL)
	assert.NoError(t, err, "Cannot parse URL")
	proxy, _ := newProxy(proxyConfig{hostAndPort: serverURL.Host, protocol: "http", readOnly: true})
	proxyServer := httptest.NewServer(proxy)
	defer proxyServer.Close()

	// Gets should be allowed
	_, err = http.Get(fmt.Sprintf("%s%s", proxyServer.URL, expectedURI))
	assert.NoError(t, err, "Failed to get URL")
	assert.True(t, atomic.LoadUint32(&handlerCalled) == 1, "Server wasn't called")

	// Posts should not be allowed
	resp, err := http.Post(fmt.Sprintf("%s%s", proxyServer.URL, expectedURI), "", nil)
	assert.NoError(t, err, "Failed to get URL")
	assert.Equal(t, resp.StatusCode, http.StatusServiceUnavailable)
	assert.True(t, atomic.LoadUint32(&handlerCalled) == 1, "Server was called")
}

type testGRPCServer struct {
	*httpgrpc_server.Server
	URL        string
	grpcServer *grpc.Server
}

func newTestGRPCServer(handler http.Handler) (*testGRPCServer, error) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}

	server := &testGRPCServer{
		Server:     httpgrpc_server.NewServer(middleware.Tracer{}.Wrap(handler)),
		grpcServer: grpc.NewServer(),
		URL:        "direct://" + lis.Addr().String(),
	}

	httpgrpc.RegisterHTTPServer(server.grpcServer, server.Server)
	go server.grpcServer.Serve(lis)

	return server, nil
}

func TestProxyGRPCTracing(t *testing.T) {
	jaeger := jaegercfg.Configuration{}
	closer, err := jaeger.InitGlobalTracer("test")
	defer closer.Close()
	require.NoError(t, err)

	server, err := newTestGRPCServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			span := opentracing.SpanFromContext(r.Context())
			if span == nil {
				w.WriteHeader(418)
				fmt.Fprint(w, "Span missing!")
			} else {
				w.WriteHeader(200)
				fmt.Fprint(w, span.BaggageItem("name"))
			}
		}),
	)

	require.NoError(t, err)
	defer server.grpcServer.GracefulStop()

	// Setup a proxy server pointing at the server
	proxy, err := newProxy(proxyConfig{grpcHost: server.URL, protocol: "http"})
	require.NoError(t, err)
	proxyServer := httptest.NewServer(
		middleware.Merge(
			middleware.Func(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					// Act like authfe and inject the orgID header
					ctx := user.InjectOrgID(r.Context(), "fakeOrg")
					err := user.InjectOrgIDIntoHTTPRequest(ctx, r)

					// Either start or continue a span
					span := opentracing.SpanFromContext(ctx)
					if span == nil {
						span, ctx = opentracing.StartSpanFromContext(ctx, "Test")
					}
					r = r.WithContext(ctx)
					// Add some baggage to be sure everything is propagated
					span.SetBaggageItem("name", "world")

					require.NoError(t, err)
					next.ServeHTTP(w, r)
				})
			}),
		).Wrap(proxy),
	)

	defer proxyServer.Close()
	ctx := context.Background()
	ctx = user.InjectOrgID(ctx, "fakeOrg")

	req, err := http.NewRequest("GET", fmt.Sprintf("%s%s", proxyServer.URL, "/foo"), nil)
	require.NoError(t, err)
	req = req.WithContext(ctx)

	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err, "error making request")

	body := make([]byte, 50)
	c, _ := resp.Body.Read(body)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "world", string(body[:c]))
}
