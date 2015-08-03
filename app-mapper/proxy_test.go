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

	"github.com/stretchr/testify/assert"
	"golang.org/x/net/websocket"
)

func testProxy(t *testing.T, targetHandler http.Handler, a authenticator, testFunc func(proxyHost string)) {
	targetTestServer := httptest.NewServer(targetHandler)
	defer targetTestServer.Close()
	parsedTargetURL, err := url.Parse(targetTestServer.URL)
	assert.NoError(t, err, "Cannot parse targetTestServer URL")

	// Set a test server for the proxy (required for Hijack to work)
	m := &constantMapper{parsedTargetURL.Host}
	proxyTestServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		appProxy(a, m, w, r)
	}))
	defer proxyTestServer.Close()

	parsedProxyURL, err := url.Parse(proxyTestServer.URL)
	assert.NoError(t, err, "Cannot parse proxyTestServer URL")
	testFunc(parsedProxyURL.Host)
}

// Test that a request sent to the proxy is received by the other end
// and that the reply is passed back
func testHTTPRequestTransparency(t *testing.T, req *http.Request) {
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
		w.Write(targetResponse)
	})

	testFunc := func(proxyHost string) {
		// Tweak and make request to the proxy
		// Inject target proxy hostname
		req.URL.Host = proxyHost
		// Explicitly set some headers which net/http/client injects if unset
		req.Header.Set("User-Agent", "Foo")
		req.Header.Set("Accept-Encoding", "gzip")
		client := &http.Client{}
		res, err := client.Do(req)
		defer res.Body.Close()
		assert.NoError(t, err, "Cannot make test request")

		// Check that everything was received as expected
		requestEqual(t, req, targetRecordedReq, "Request mismatch")
		assert.Equal(t, http.StatusOK, res.StatusCode, "Response status mismatch")
		body, err := ioutil.ReadAll(res.Body)
		assert.NoError(t, err, "Cannot read response body")
		assert.Equal(t, targetResponse, body, "Response body mismatch")
	}

	testProxy(t, targetHandler, &mockAuthenticator{}, testFunc)
}

func copyRequest(t *testing.T, req *http.Request) *http.Request {
	dump, err := httputil.DumpRequest(req, true)
	assert.NoError(t, err, "Cannot dump request body")
	clone, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(dump)))
	assert.NoError(t, err, "Cannot parse request")
	clone.ContentLength = req.ContentLength
	return clone
}

func requestEqual(t *testing.T, expected *http.Request, actual *http.Request, msg string) {
	assert.Equal(t, expected.Method, actual.Method, msg+": method")
	assert.Equal(t, expected.ContentLength, actual.ContentLength, msg+": content length")
	assert.Equal(t, expected.Header, actual.Header, msg+": headers")
	assert.Equal(t, expected.TransferEncoding, actual.TransferEncoding, msg+": transfer encoding")
	if expected.ContentLength != 0 {
		expectedBody, err := ioutil.ReadAll(expected.Body)
		assert.NoError(t, err, "Cannot read expected request Body")
		actualBody, err := ioutil.ReadAll(actual.Body)
		assert.NoError(t, err, "Cannot read actual request Body")
		assert.Equal(t, expectedBody, actualBody, msg+": body")
	}
}

func TestProxyGet(t *testing.T) {
	req, err := http.NewRequest("GET", "http://example.com/request?arg1=foo&arg2=bar", nil)
	assert.NoError(t, err, "Cannot create request")
	testHTTPRequestTransparency(t, req)
}

func TestProxyPost(t *testing.T) {
	req, err := http.NewRequest("POST", "http://example.com/request?arg1=foo&arg2=bar",
		strings.NewReader("z=post&both=y&prio=2&empty="))
	assert.NoError(t, err, "Cannot create request")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; param=value")
	testHTTPRequestTransparency(t, req)
}

type AuthenticatorFunc func(r *http.Request) (authenticatorResponse, error)

func (f AuthenticatorFunc) authenticate(r *http.Request) (authenticatorResponse, error) {
	return f(r)
}

func TestUnauthorized(t *testing.T) {
	// The target handler should never be reached
	targetHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Fail(t, "Target contacted by proxy")
	})

	var authenticatorError error
	failingAuthenticator := AuthenticatorFunc(func(r *http.Request) (authenticatorResponse, error) {
		return authenticatorResponse{}, authenticatorError
	})

	testFunc := func(proxyHost string) {
		authErrToProxyStatus := map[error]int{
			unauthorized{http.StatusUnauthorized}:        http.StatusUnauthorized,
			unauthorized{http.StatusBadRequest}:          http.StatusUnauthorized,
			unauthorized{http.StatusInternalServerError}: http.StatusUnauthorized,
			errors.New("Whatever"):                       http.StatusBadGateway,
		}

		url := "http://" + proxyHost + "/request?arg1=foo&arg2=bar"
		for authErr, expectedProxyStatus := range authErrToProxyStatus {
			authenticatorError = authErr
			res, err := http.Get(url)
			assert.NoError(t, err, "Cannot send request")
			defer res.Body.Close()
			assert.Equal(t, expectedProxyStatus, res.StatusCode,
				"Unexpected proxy response status with failing authenticator")
		}
	}

	testProxy(t, targetHandler, failingAuthenticator, testFunc)
}

func TestProxyWebSocket(t *testing.T) {
	// Use a websocket echo server in the target
	targetHandler := websocket.Handler(func(ws *websocket.Conn) {
		io.Copy(ws, ws)
	})

	testFunc := func(proxyHost string) {
		// Establish a websocket connection with the proxy
		ws, err := websocket.Dial("ws://"+proxyHost+"/request?arg1=foo&arg2=bar", "", "http://example.com")
		assert.NoError(t, err, "Cannot dial websocket server")
		defer ws.Close()

		// We should receive back exactly what we send
		messageToSend := "This is a test message"
		var messageToReceive string
		for i := 0; i < 100; i++ {
			websocket.Message.Send(ws, messageToSend)
			websocket.Message.Receive(ws, &messageToReceive)
			assert.Equal(t, messageToSend, messageToReceive)
			messageToReceive = ""
		}
	}

	testProxy(t, targetHandler, &mockAuthenticator{}, testFunc)
}
