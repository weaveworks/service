package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/weaveworks/service/dashboard-api/aws"
	"github.com/weaveworks/service/dashboard-api/dashboard"
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

// Test that the endpoint returns 404 when there are no metrics
func TestGetAWSDashboardsNoMetrics(t *testing.T) {
	api := setupMock(t)

	req := httptest.NewRequest("GET", fmt.Sprintf("%s/api/dashboard/aws/%s/dashboards", baseURL, aws.ELB.Type), nil)
	w := httptest.NewRecorder()
	api.handler.ServeHTTP(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestGetAWSDashboardsRDS(t *testing.T) {
	api := setupMock(t)
	err := dashboard.Init()
	assert.NoError(t, err)

	req := httptest.NewRequest("GET", fmt.Sprintf("%s/api/dashboard/aws/%s/dashboards", baseURL, aws.RDS.Type), nil)
	w := httptest.NewRecorder()
	api.handler.ServeHTTP(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	bytes, err := ioutil.ReadAll(resp.Body)
	assert.NoError(t, err)
	var body getDashboardsResponse
	assert.NoError(t, json.Unmarshal(bytes, &body))
	assert.Equal(t, expectedRDSDashboard, body.Dashboards)
}

var expectedRDSDashboard = []dashboard.Dashboard{
	{
		ID:   "aws-rds",
		Name: "RDS",
		Sections: []dashboard.Section{
			{
				Name: "System",
				Rows: []dashboard.Row{
					{
						Panels: []dashboard.Panel{
							{
								Title:    "CPU utilization",
								Optional: false,
								Help:     "",
								Type:     "line",
								Unit: dashboard.Unit{
									Format:      "numeric",
									Scale:       0,
									Explanation: "",
								},
								Query: "sum(aws_rds_cpuutilization_average{kubernetes_namespace='weave',_weave_service='cloudwatch-exporter',dbinstance_identifier=~'.+'}) by (dbinstance_identifier)",
							},
							{
								Title:    "Available RAM",
								Optional: false,
								Help:     "",
								Type:     "line",
								Unit: dashboard.Unit{
									Format:      "bytes",
									Scale:       0,
									Explanation: "",
								},
								Query: "sum(aws_rds_freeable_memory_average{kubernetes_namespace='weave',_weave_service='cloudwatch-exporter',dbinstance_identifier=~'.+'}) by (dbinstance_identifier)",
							},
						},
					},
				},
			},
			{
				Name: "Database",
				Rows: []dashboard.Row{
					{
						Panels: []dashboard.Panel{
							{
								Title:    "Number of connections in use",
								Optional: false,
								Help:     "",
								Type:     "line",
								Unit: dashboard.Unit{
									Format:      "numeric",
									Scale:       0,
									Explanation: "",
								},
								Query: "sum(aws_rds_database_connections_average{kubernetes_namespace='weave',_weave_service='cloudwatch-exporter',dbinstance_identifier=~'.+'}) by (dbinstance_identifier)",
							},
						},
					},
				},
			},
			{
				Name: "Disk",
				Rows: []dashboard.Row{
					{
						Panels: []dashboard.Panel{
							{
								Title:    "Read IOPS",
								Optional: false,
								Help:     "",
								Type:     "line",
								Unit: dashboard.Unit{
									Format:      "numeric",
									Scale:       0,
									Explanation: "",
								},
								Query: "sum(aws_rds_read_iops_average{kubernetes_namespace='weave',_weave_service='cloudwatch-exporter',dbinstance_identifier=~'.+'}) by (dbinstance_identifier)",
							},
							{
								Title:    "Write IOPS",
								Optional: false,
								Help:     "",
								Type:     "line",
								Unit: dashboard.Unit{
									Format:      "numeric",
									Scale:       0,
									Explanation: "",
								},
								Query: "sum(aws_rds_write_iops_average{kubernetes_namespace='weave',_weave_service='cloudwatch-exporter',dbinstance_identifier=~'.+'}) by (dbinstance_identifier)",
							},
						},
					},
				},
			},
		},
	},
}
