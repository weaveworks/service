package main

import (
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	users "github.com/weaveworks/service/users/client"
)

func TestRoutes(t *testing.T) {
	// Create mock config
	cfg := Config{
		launcherServiceExternalHost: "get.weave.works",
	}
	authenticator, err := users.New("mock", "users:4772", users.CachingClientConfig{})
	assert.NoError(t, err, "Error creating the authenticator")

	// Initialize all of the proxies with mock which wrties the proxy name in the body
	for name, proxyCfg := range cfg.proxies() {
		mockProxyCfg := &proxyConfig{
			name:     name,
			protocol: "mock",
		}
		handler, err := newProxy(*mockProxyCfg)
		assert.NoError(t, err, "Error creating the proxy")
		proxyCfg.Handler = handler
	}

	// Create the routes handler
	handler, err := routes(cfg, authenticator, nil, nil)
	assert.NoError(t, err, "Error creating the routes handler")

	tests := []struct {
		url    string
		method string
		body   io.Reader

		expectedProxyName string
		expectedCode      int
		expectedLocation  string
	}{
		// HostnameSpecific
		{url: "https://get.weave.works/", expectedProxyName: "launcher-service"},
		{url: "https://get.weave.works/bootstrap", expectedProxyName: "launcher-service"},

		// Weave Cloud
		{url: "/", expectedProxyName: "ui-server"},
		{url: "/launch/k8s", expectedProxyName: "launch-generator"},
		{url: "/k8s", expectedProxyName: "launch-generator"},
		{url: "/api/ui/metrics", expectedProxyName: "ui-metrics"},
		{url: "/api/users", expectedProxyName: "users"},
		// GCP redirect
		{url: "/subscribe-via/gcp?retain=me", method: "POST", body: strings.NewReader(`x-gcp-marketplace-token=foo`),
			expectedCode: 302, expectedLocation: "/subscribe-via/gcp?retain=me&x-gcp-marketplace-token=foo"},
		{url: "/login-via/gcp?retain=me", method: "POST", body: strings.NewReader(`x-gcp-marketplace-token=foo`),
			expectedCode: 302, expectedLocation: "/login-via/gcp?retain=me&x-gcp-marketplace-token=foo"},
	}

	for _, tc := range tests {
		t.Run(tc.method+" "+tc.url, func(t *testing.T) {
			if tc.method == "" {
				tc.method = "GET"
			}
			req, err := http.NewRequest(tc.method, tc.url, tc.body)
			if tc.method == "POST" {
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			}
			assert.NoError(t, err, "Error creating the request")

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
			resp := rr.Result()

			if tc.expectedCode != 0 {
				assert.Equal(t, tc.expectedCode, rr.Code)
			}
			if tc.expectedProxyName != "" {
				body, _ := ioutil.ReadAll(resp.Body)
				resp.Body.Close()
				assert.Equal(t, tc.expectedProxyName, string(body))
			}
			if tc.expectedLocation != "" {
				assert.Equal(t, tc.expectedLocation, rr.Header().Get("Location"))
			}
		})
	}
}

func TestStripSetCookieHeader(t *testing.T) {
	tests := []struct {
		url      string
		prefixes []string
		stripped bool
	}{
		{"https://weave.test/test", []string{"/test"}, true},
		{"https://weave.test/test", []string{}, false},
		{"https://weave.test/foo", []string{"/test"}, false},
	}

	for _, tc := range tests {
		mw := stripSetCookieHeader{prefixes: tc.prefixes}
		req, err := http.NewRequest("GET", tc.url, nil)
		assert.NoError(t, err)

		rec := httptest.NewRecorder()
		mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.SetCookie(w, &http.Cookie{Name: "foo", Value: "bar"})
			w.WriteHeader(200)
			w.Write([]byte{})
		})).ServeHTTP(rec, req)
		resp := rec.Result()
		resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, tc.stripped, resp.Header.Get("Set-Cookie") == "")
	}
}
