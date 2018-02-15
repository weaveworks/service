package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

func setupMock(t *testing.T) *API {
	cfg := &config{}

	wd, err := os.Getwd()
	assert.Nil(t, err)

	cfg.prometheus.uri = fmt.Sprintf("mock://%s", wd)
	api, err := newAPI(cfg)
	assert.Nil(t, err)

	return api
}

func makeGetServiceMetricsURL(ns, service string) string {
	const baseURL = "http://api.dashboard.svc.cluster.local"

	return fmt.Sprintf("%s/api/dashboard/services/%s/%s/metrics", baseURL, ns, service)
}

// Test that the endpoint returns { metrics: [] } then there are no metrics to report
func TestGetServiceMetricsNoMetrics(t *testing.T) {
	api := setupMock(t)

	req := httptest.NewRequest("GET", makeGetServiceMetricsURL("foo", "bar"), nil)
	w := httptest.NewRecorder()
	api.handler.ServeHTTP(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	gotBytes, err := ioutil.ReadAll(resp.Body)
	assert.Nil(t, err)
	assert.Equal(t, "{\"metrics\":[]}\n", string(gotBytes))
}

func TestGetServiceMetrics(t *testing.T) {
	api := setupMock(t)
	ns := "notification"
	service := "eventmanager"

	req := httptest.NewRequest("GET", makeGetServiceMetricsURL(ns, service), nil)
	w := httptest.NewRecorder()
	api.handler.ServeHTTP(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	gotBytes, err := ioutil.ReadAll(resp.Body)
	assert.Nil(t, err)

	// We need to sort the results for deterministic comparison with the golden
	// data (we're iterating over a map to get the names of metrics)
	got := getServiceMetricsResponse{}
	err = json.Unmarshal(gotBytes, &got)
	assert.Nil(t, err)
	sort.Strings(got.Metrics)

	golden := filepath.Join("testdata", fmt.Sprintf("%s-%s-%s.golden", t.Name(), ns, service))
	if *update {
		data, err := json.Marshal(got)
		assert.Nil(t, err)
		ioutil.WriteFile(golden, data, 0644)
	}

	expectedBytes, err := ioutil.ReadFile(golden)
	assert.Nil(t, err)
	expected := getServiceMetricsResponse{}
	err = json.Unmarshal(expectedBytes, &expected)
	assert.Nil(t, err)

	assert.Equal(t, expected, got)
}
