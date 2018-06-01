package main

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetAWSResources(t *testing.T) {
	api := setupMock(t)
	req := httptest.NewRequest("GET", "http://api.dashboard.svc.cluster.local/api/dashboard/aws/resources", nil)
	w := httptest.NewRecorder()
	api.handler.ServeHTTP(w, req)
	var resp getAWSResourcesResponse
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, getAWSResourcesResponse([]resource{
		{Type: "RDS", Name: "prod-billing-db"},
		{Type: "RDS", Name: "prod-configs-vpc-database"},
		{Type: "RDS", Name: "prod-fluxy-vpc-database"},
		{Type: "RDS", Name: "prod-notification-configs-vpc-database"},
		{Type: "RDS", Name: "prod-users-vpc-database"},
	}), resp)
}

func TestToSnakeCase(t *testing.T) {
	assert.Equal(t, "dbinstance_identifier", toSnakeCase("DBInstanceIdentifier"))
	assert.Equal(t, "queue_name", toSnakeCase("QueueName"))
	assert.Equal(t, "load_balancer_name", toSnakeCase("LoadBalancerName"))
	assert.Equal(t, "function_name", toSnakeCase("FunctionName"))
}
