package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/weaveworks/service/dashboard-api/aws"
)

const baseURL = "http://api.dashboard.svc.cluster.local"

func makeGetServiceDashboardsURL(ns, service string) string {
	return fmt.Sprintf("%s/api/dashboard/services/%s/%s/dashboards", baseURL, ns, service)
}

// Test that the endpoint returns 404 when there are no metrics
func TestGetServiceDashboardsNoMetrics(t *testing.T) {
	api := setupMock(t)

	req := httptest.NewRequest("GET", makeGetServiceDashboardsURL("foo", "bar"), nil)
	w := httptest.NewRecorder()
	api.handler.ServeHTTP(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

// Test that the endpoint returns 404 when there are no metrics
func TestGetAWSDashboardNoMetrics(t *testing.T) {
	api := setupMock(t)

	req := httptest.NewRequest("GET", fmt.Sprintf("%s/api/dashboard/aws/%s/%s/dashboards", baseURL, aws.RDS.Type, "bar"), nil)
	w := httptest.NewRecorder()
	api.handler.ServeHTTP(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}
