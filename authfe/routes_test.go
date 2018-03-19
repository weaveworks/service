package main

import (
	"net/http"
	"net/http/httptest"
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
		url               string
		expectedProxyName string
	}{
		// HostnameSpecific
		{"https://get.weave.works/", "launcher-service"},
		{"https://get.weave.works/bootstrap", "launcher-service"},

		// Weave Cloud
		{"/", "ui-server"},
		{"/launch/k8s", "launch-generator"},
		{"/k8s", "launch-generator"},
		{"/api/ui/metrics", "ui-metrics"},
		{"/api/users", "users"},
	}

	for _, tc := range tests {
		req, err := http.NewRequest("GET", tc.url, nil)
		assert.NoError(t, err, "Error creating the request")

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, tc.expectedProxyName, rr.Body.String())
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
		})).ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)

		assert.Equal(t, tc.stripped, rec.Header().Get("Set-Cookie") == "")
	}
}
