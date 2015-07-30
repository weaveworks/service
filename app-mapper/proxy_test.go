package main

import (
	"bufio"
	"bytes"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func testProxyRequest(t *testing.T, req *http.Request) {
	// Set a test server to be targeted by the proxy
	var targetResponse = []byte("Hi there, this is a response")
	var targetRecordedReq *http.Request
	targetTestServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetRecordedReq = copyRequest(t, r)
		w.WriteHeader(http.StatusOK)
		w.Write(targetResponse)
	}))
	defer targetTestServer.Close()
	parsedTargetURL, err := url.Parse(targetTestServer.URL)
	assert.NoError(t, err, "Cannot parse targetTestServer URL")

	// Set a test server for the proxy (required for Hijack to work)
	a := &mockAuthenticator{}
	m := &constantMapper{parsedTargetURL.Host}
	proxyTestServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		appProxy(a, m, w, r)
	}))
	// TODO: for some reason the test hangs if we close the proxyTestServer
	//defer proxyTestServer.Close()

	// Tweak and make request to the proxy
	// Inject target proxy hostname
	parsedProxyURL, err := url.Parse(proxyTestServer.URL)
	assert.NoError(t, err, "Cannot parse proxyTestServer URL")
	req.URL.Host = parsedProxyURL.Host
	// Explicitly set some headers which golang injects if not unset
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

func copyRequest(t *testing.T, req *http.Request) *http.Request {
	dump, err := httputil.DumpRequest(req, true)
	assert.NoError(t, err, "Cannot dump request body")
	copy, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(dump)))
	assert.NoError(t, err, "Cannot parse request")
	copy.ContentLength = req.ContentLength
	return copy
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
	req, err := http.NewRequest("GET", "http://example.com/api/app/request?arg1=foo&arg2=bar", nil)
	assert.NoError(t, err, "Cannot create request")
	testProxyRequest(t, req)
}

func TestProxyPost(t *testing.T) {
	req, err := http.NewRequest("POST", "http://example.com/api/app/request?arg1=foo&arg2=bar",
		strings.NewReader("z=post&both=y&prio=2&empty="))
	assert.NoError(t, err, "Cannot create request")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; param=value")
	testProxyRequest(t, req)
}
