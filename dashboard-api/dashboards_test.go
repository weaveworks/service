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

func makeGetAWSDashboardsURL(awsType, resourceName string) string {
	return fmt.Sprintf("%s/api/dashboard/aws/%s/%s/dashboards", baseURL, awsType, resourceName)
}

// Test that the endpoint returns 404 when there are no metrics
func TestGetAWSDashboardsNoMetrics(t *testing.T) {
	api := setupMock(t)

	req := httptest.NewRequest("GET", makeGetAWSDashboardsURL(aws.RDS, "bar"), nil)
	w := httptest.NewRecorder()
	api.handler.ServeHTTP(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}
