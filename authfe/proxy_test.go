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

	gorillaws "github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/websocket"
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
	proxyServer := httptest.NewServer(newProxy(wsURL.Host))
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
	proxyServer := httptest.NewServer(newProxy(serverURL.Host))
	defer proxyServer.Close()

	_, err = http.Get(fmt.Sprintf("http://%s%s", serverURL.Host, expectedURI))
	assert.NoError(t, err, "Failed to get URL")
	assert.True(t, atomic.LoadUint32(&handlerCalled) == 1, "Server wasn't called")
}
