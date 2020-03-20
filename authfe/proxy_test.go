package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-kit/kit/sd"
	gorillaws "github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/websocket"

	"github.com/weaveworks/service/authfe/balance"
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

// Test proxying to more than one backend
func TestProxyMultiServer(t *testing.T) {
	const nCalls = 12

	roundRobin := func(hosts []string) balance.Balancer {
		return balance.NewRoundRobin(sd.FixedInstancer(hosts))
	}

	for _, test := range []struct {
		name           string
		serverSpeed    []int
		factory        func([]string) balance.Balancer
		expectedCalls  []int
		expectedStatus int
	}{
		{
			name:           "no servers",
			factory:        roundRobin,
			expectedStatus: 500,
		},
		{
			name:           "two same-speed servers",
			serverSpeed:    []int{1, 1},
			factory:        roundRobin,
			expectedCalls:  []int{nCalls / 2, nCalls / 2},
			expectedStatus: 200,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			hosts := make([]string, len(test.serverSpeed))
			handlerCalled := make([]uint32, len(test.serverSpeed))
			// Set up N dummy servers that count calls and wait a short time
			for i, speed := range test.serverSpeed {
				i := i // new copy for closure capture
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					atomic.AddUint32(&handlerCalled[i], 1)
					time.Sleep(time.Duration(speed) * time.Millisecond)
				}))
				defer server.Close()
				serverURL, err := url.Parse(server.URL)
				assert.NoError(t, err)
				hosts[i] = serverURL.Host
			}

			balancer := test.factory(hosts[:])
			defer balancer.Close()
			proxy, _ := newProxy(proxyConfig{
				balancer: balancer,
				protocol: "http",
			})
			proxyServer := httptest.NewServer(proxy)
			defer proxyServer.Close()

			// Now make N calls through the proxy
			for i := 0; i < nCalls; i++ {
				resp, err := http.Get(proxyServer.URL)
				assert.NoError(t, err)
				assert.Equal(t, test.expectedStatus, resp.StatusCode)
			}
			// Check we got the expected number of calls
			for i, expectedCalls := range test.expectedCalls {
				assert.Equal(t, uint32(expectedCalls), atomic.LoadUint32(&handlerCalled[i]), "wrong number of calls")
			}
		})
	}
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
