package main

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	gorillaws "github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	scope "github.com/weaveworks/scope/xfer"
	"golang.org/x/net/websocket"
)

func testProxy(t *testing.T, targetHandler http.Handler, isTargetReady *bool, a authenticator, testFunc func(proxyHost string, pms *probeMemStorage)) {
	targetTestServer := httptest.NewServer(targetHandler)
	defer targetTestServer.Close()
	parsedTargetURL, err := url.Parse(targetTestServer.URL)
	require.NoError(t, err, "Cannot parse targetTestServer URL")

	// Set a test server for the proxy (required for Hijack to work)
	m := &constantMapper{parsedTargetURL.Host, isTargetReady}
	p := &probeMemStorage{}
	router := mux.NewRouter()
	newProxy(a, m, p).registerHandlers(router)
	proxyTestServer := httptest.NewServer(router)
	defer proxyTestServer.Close()

	parsedProxyURL, err := url.Parse(proxyTestServer.URL)
	require.NoError(t, err, "Cannot parse proxyTestServer URL")
	testFunc(parsedProxyURL.Host, p)
}

// Test that a request sent to the proxy is received by the other end
// and that the reply is passed back
func testHTTPRequest(t *testing.T, req *http.Request) {
	// Set a test server to be targeted by the proxy
	var targetResponse = []byte("Hi there, this is a response")
	var targetRecordedReq *http.Request
	targetHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetRecordedReq = copyRequest(t, r)
		// Close the connection after replying to let go of the proxy
		// (otherwise, the test will hang because the DefaultTransport
		//  caches clients, and doesn't explicitly close them)
		w.Header().Set("Connection", "close")
		w.WriteHeader(http.StatusOK)
		_, err := w.Write(targetResponse)
		assert.NoError(t, err, "Couldn't write response body")
	})

	authenticator := authenticatorFunc(func(r *http.Request, orgName string) (authenticatorResponse, error) {
		return authenticatorResponse{"somePersistentInternalID"}, nil
	})

	testFunc := func(proxyHost string, pms *probeMemStorage) {
		// Tweak and make request to the proxy
		// Inject target proxy hostname
		req.URL.Host = proxyHost
		// Explicitly set some headers which net/http/client injects if unset
		req.Header.Set("User-Agent", "Foo")
		req.Header.Set("Accept-Encoding", "gzip")
		res, err := http.DefaultClient.Do(req)
		defer res.Body.Close()
		require.NoError(t, err, "Cannot make test request")

		// Check that everything was received as expected
		require.NotNil(t, targetRecordedReq, "target didn't receive request")
		requestEqual(t, req, targetRecordedReq, "Request mismatch")
		assert.Equal(t, http.StatusOK, res.StatusCode, "Response status mismatch")
		body, err := ioutil.ReadAll(res.Body)
		assert.NoError(t, err, "Cannot read response body")
		assert.Equal(t, targetResponse, body, "Response body mismatch")
	}

	testProxy(t, targetHandler, nil, authenticator, testFunc)
}

func copyRequest(t *testing.T, req *http.Request) *http.Request {
	dump, err := httputil.DumpRequest(req, true)
	require.NoError(t, err, "Cannot dump request body")
	clone, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(dump)))
	require.NoError(t, err, "Cannot parse request")
	clone.ContentLength = req.ContentLength
	return clone
}

func requestEqual(t *testing.T, expected *http.Request, actual *http.Request, msg string) {
	assert.Equal(t, expected.Method, actual.Method, msg+": method")
	assert.Equal(t, expected.ContentLength, actual.ContentLength, msg+": content length")
	// Leave "X-Forwarded-For" out of the comparison since it's legitimately injected by the proxy
	headerToCompare := make(http.Header)
	copyHeader(headerToCompare, actual.Header)
	delete(headerToCompare, "X-Forwarded-For")
	assert.Equal(t, expected.Header, headerToCompare, msg+": headers")
	assert.Equal(t, expected.TransferEncoding, actual.TransferEncoding, msg+": transfer encoding")
	if expected.ContentLength != 0 {
		expectedBody, err := ioutil.ReadAll(expected.Body)
		assert.NoError(t, err, "Cannot read expected request Body")
		actualBody, err := ioutil.ReadAll(actual.Body)
		assert.NoError(t, err, "Cannot read actual request Body")
		assert.Equal(t, expectedBody, actualBody, msg+": body")
	}
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

func TestProxyGet(t *testing.T) {
	req, err := http.NewRequest("GET", "http://example.com/api/app/somePublicOrgName/request?arg1=foo&arg2=bar", nil)
	require.NoError(t, err, "Cannot create request")
	testHTTPRequest(t, req)

	req, err = http.NewRequest("GET", "http://example.com/api/report?arg1=foo&arg2=bar", nil)
	require.NoError(t, err, "Cannot create request")
	testHTTPRequest(t, req)

}

func TestProxyPost(t *testing.T) {
	req, err := http.NewRequest("POST", "http://example.com/api/app/somePublicOrgName/request?arg1=foo&arg2=bar",
		strings.NewReader("z=post&both=y&prio=2&empty="))
	require.NoError(t, err, "Cannot create request")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; param=value")
	testHTTPRequest(t, req)

	req, err = http.NewRequest("POST", "http://example.com/api/report?arg1=foo&arg2=bar",
		strings.NewReader("z=post&both=y&prio=2&empty="))
	require.NoError(t, err, "Cannot create request")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; param=value")
	testHTTPRequest(t, req)

}

func TestProxyStrictSlash(t *testing.T) {
	// Set a test server to be targeted by the proxy
	reachedTarget := false
	targetHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reachedTarget = true
		// Close the connection after replying to let go of the proxy
		// (otherwise, the test will hang because the DefaultTransport
		//  caches clients, and doesn't explicitly close them)
		w.Header().Set("Connection", "close")
		w.WriteHeader(http.StatusOK)
	})

	testFunc := func(proxyHost string, pms *probeMemStorage) {
		redirected := false
		client := &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				redirected = true
				return nil
			},
		}
		url := "http://" + proxyHost + "/api/app/somePublicOrgName"
		res, err := client.Get(url)
		res.Body.Close()
		require.NoError(t, err, "Cannot make test request")

		// Check that everything was received as expected
		assert.True(t, reachedTarget, "target wasn't reached")
		assert.True(t, redirected, "redirection didn't happen")
	}

	testProxy(t, targetHandler, nil, &mockAuthenticator{}, testFunc)
}

func TestProxyEncoding(t *testing.T) {
	req, err := http.NewRequest("GET", "http://example.com/api/app/some%2FPublic%2FOrgName/request?arg1=foo&arg2=bar", nil)
	require.NoError(t, err, "Cannot create request")
	testHTTPRequest(t, req)
}

func TestProxyRewrites(t *testing.T) {

	// Set a test server to be targeted by the proxy
	var reachedPath string
	targetHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reachedPath = r.URL.Path
		// Close the connection after replying to let go of the proxy
		// (otherwise, the test will hang because the DefaultTransport
		//  caches clients, and doesn't explicitly close them)
		w.Header().Set("Connection", "close")
		w.WriteHeader(http.StatusOK)
	})

	testFunc := func(proxyHost string, pms *probeMemStorage) {
		path := "/request"
		res, err := http.Get("http://" + proxyHost + "/api/app/somePublicOrgName" + path + "?arg1=foo&arg2=bar")
		defer res.Body.Close()
		require.NoError(t, err, "Cannot make test request")

		// Check that everything was received as expected
		assert.Equal(t, path, reachedPath, "unexpected rewrite")

		reachedPath = ""
		path = "/api/report"
		res, err = http.Get("http://" + proxyHost + path + "?arg1=foo&arg2=bar")
		defer res.Body.Close()
		require.NoError(t, err, "Cannot make test request")

		// Check that everything was received as expected
		assert.Equal(t, path, reachedPath, "unexpected rewrite")

	}

	testProxy(t, targetHandler, nil, &mockAuthenticator{}, testFunc)
}

func TestMultiRequest(t *testing.T) {

	targetReqCounter := 0
	targetHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetReqCounter++
		w.WriteHeader(http.StatusOK)
	})

	authenticatorReqCounter := 0
	authenticator := authenticatorFunc(func(r *http.Request, orgName string) (authenticatorResponse, error) {
		authenticatorReqCounter++
		return authenticatorResponse{"somePersistentInternalID"}, nil
	})

	testFunc := func(proxyHost string, pms *probeMemStorage) {
		url := "http://" + proxyHost + "/api/app/somePublicOrgName/request?arg1=foo&arg2=bar"
		req, err := http.NewRequest("GET", url, nil)
		require.NoError(t, err, "Cannot create request")
		req.Header.Set("Connection", "keep-alive")
		for i := 0; i < 100; i++ {
			if i == 99 {
				// Close the connection in the last request to let go of the proxy
				// (otherwise, the test will hang because the DefaultTransport
				//  caches clients, and doesn't explicitly close them)
				req.Header.Set("Connection", "close")
			}
			res, err := http.DefaultClient.Do(req)
			require.NoError(t, err, "Cannot make test request")
			res.Body.Close()
		}
		assert.Equal(t, 100, targetReqCounter, "Mismatching target requests")
		assert.Equal(t, 100, authenticatorReqCounter, "Mismatching authenticator requests")
	}

	testProxy(t, targetHandler, nil, authenticator, testFunc)
}

func TestUnauthorized(t *testing.T) {
	// The target handler should never be reached
	targetHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Fail(t, "Target contacted by proxy")
	})

	failingAuthenticator := authenticatorFunc(func(r *http.Request, orgName string) (authenticatorResponse, error) {
		return authenticatorResponse{}, errors.New("PhonyError")
	})

	testFunc := func(proxyHost string, pms *probeMemStorage) {
		for _, url := range []string{
			"http://" + proxyHost + "/api/app/somePublicOrgName/request?arg1=foo&arg2=bar",
			"http://" + proxyHost + "/api/report?arg1=foo&arg2=bar",
		} {
			res, err := http.Get(url)
			assert.NoError(t, err, "Cannot send request")
			defer res.Body.Close()
			assert.NotEqual(t, http.StatusOK, res.StatusCode,
				"Unexpected OK status with failing authenticator")
		}
	}

	testProxy(t, targetHandler, nil, failingAuthenticator, testFunc)
}

func TestProxyWebSocket(t *testing.T) {
	// Use a websocket echo server in the target
	targetHandler := websocket.Handler(func(ws *websocket.Conn) {
		_, err := io.Copy(ws, ws)
		assert.NoError(t, err, "Target handler copy failed")
	})

	testFunc := func(proxyHost string, pms *probeMemStorage) {
		// Establish a websocket connection with the proxy
		ws, err := websocket.Dial("ws://"+proxyHost+"/api/app/somePublicOrgName/request?arg1=foo&arg2=bar", "", "http://example.com")
		require.NoError(t, err, "Cannot dial websocket server")
		defer ws.Close()

		// We should receive back exactly what we send
		messageToSend := "This is a test message"
		var messageToReceive string
		for i := 0; i < 100; i++ {
			err = websocket.Message.Send(ws, messageToSend)
			require.NoError(t, err, "Error sending message")
			err = websocket.Message.Receive(ws, &messageToReceive)
			require.NoError(t, err, "Error receiving message")
			require.Equal(t, messageToSend, messageToReceive, "Unexpected echoed message")
			messageToReceive = ""
		}
	}

	testProxy(t, targetHandler, nil, &mockAuthenticator{}, testFunc)
}

// Regression test for "App-mapper proxy leaks file descriptors" https://github.com/weaveworks/service/issues/253
func TestProxyWebSocketNoServerSideLeak(t *testing.T) {
	serverExited := false

	// Use a websocket echo server in the target
	targetHandler := websocket.Handler(func(ws *websocket.Conn) {
		_, err := io.Copy(ws, ws)
		assert.NoError(t, err, "Target handler copy failed")
		serverExited = true
	})

	testFunc := func(proxyHost string, pms *probeMemStorage) {
		// Establish a websocket connection with the proxy
		// Use the gorilla websocket library since its Conn.Close()
		// function doesn't send the Websocket close message (not giving
		// the server an opportunity to close the connection and forcing
		// the Proxy to explicitly do it)
		ws, _, err := gorillaws.DefaultDialer.Dial(
			"ws://"+proxyHost+"/api/app/somePublicOrgName/request?arg1=foo&arg2=bar",
			http.Header{"Origin": {"http://example.com"}},
		)
		require.NoError(t, err, "Cannot dial websocket server")

		// We should receive back exactly what we send
		messageToSend := []byte("This is a test message")
		err = ws.WriteMessage(gorillaws.TextMessage, messageToSend)
		require.NoError(t, err, "Error sending message")
		msgType, messageToReceive, err := ws.ReadMessage()
		require.NoError(t, err, "Error receiving message")
		assert.Equal(t, gorillaws.TextMessage, msgType, "Unexpected message type")
		require.Equal(t, messageToSend, messageToReceive, "Unexpected echoed message")

		// When we close the client, the proxy should also close the server-side connection
		ws.Close()
	}

	testProxy(t, targetHandler, nil, &mockAuthenticator{}, testFunc)
	assert.True(t, serverExited, "Server didn't exit")
}

func TestProbeLogging(t *testing.T) {
	// Set a test server to be targeted by the proxy
	targetHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Close the connection after replying to let go of the proxy
		// (otherwise, the test will hang because the DefaultTransport
		//  caches clients, and doesn't explicitly close them)
		w.Header().Set("Connection", "close")
		w.WriteHeader(http.StatusOK)
	})

	testFunc := func(proxyHost string, pms *probeMemStorage) {
		const probeID = "probeIDValue"

		req, err := http.NewRequest("GET", "http://"+proxyHost+"/api/report?arg1=foo&arg2=bar", nil)
		require.NoError(t, err, "Cannot create request")
		req.Header.Add(scope.ScopeProbeIDHeader, probeID)
		res, err := http.DefaultClient.Do(req)
		defer res.Body.Close()

		require.Equal(t, len(pms.memProbes), 1, "Probe wasn't logged")
		require.Equal(t, probeID, pms.memProbes[0].ID, "Mismatching Probe ID")

	}

	testProxy(t, targetHandler, nil, &mockAuthenticator{}, testFunc)
}

func TestConfirmAppIsReady(t *testing.T) {
	var (
		targetReached bool
		isTargetReady bool
	)

	// Set a test server to be targeted by the proxy
	targetHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetReached = true
		// Close the connection after replying to let go of the proxy
		// (otherwise, the test will hang because the DefaultTransport
		//  caches clients, and doesn't explicitly close them)
		w.Header().Set("Connection", "close")
		w.WriteHeader(http.StatusOK)
	})

	testFunc := func(proxyHost string, pms *probeMemStorage) {
		for _, url := range []string{
			"http://" + proxyHost + "/api/app/somePublicOrgName/request?arg1=foo&arg2=bar",
			"http://" + proxyHost + "/api/report?arg1=foo&arg2=bar",
		} {
			// Make provisioner report that the target is not ready
			isTargetReady = false
			// Reset target flag
			targetReached = false
			res, err := http.Get(url)
			require.NoError(t, err, "Cannot make test request")
			res.Body.Close()

			assert.Equal(t, http.StatusServiceUnavailable, res.StatusCode, "Should report service unavailable when the app is not ready")
			assert.False(t, targetReached, "Shouldn't reach the app if it's not ready")

			// Make provisioner report that the target ready
			isTargetReady = true
			// Reset target flag
			targetReached = false
			res, err = http.Get(url)
			require.NoError(t, err, "Cannot make test request")
			res.Body.Close()

			assert.Equal(t, http.StatusOK, res.StatusCode, "Should forward whatever the target reported")
			assert.True(t, targetReached, "The target should be reached since the app is ready")

		}
	}

	testProxy(t, targetHandler, &isTargetReady, &mockAuthenticator{}, testFunc)
}
