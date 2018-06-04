package main

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/common/model"

	"github.com/stretchr/testify/assert"
)

func TestGetAWSResources(t *testing.T) {
	api := setupMock(t)
	req := httptest.NewRequest("GET", "http://api.dashboard.svc.cluster.local/api/dashboard/aws/resources", nil)
	w := httptest.NewRecorder()
	api.handler.ServeHTTP(w, req)
	var resp []resources
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, []resources{
		{
			Type:     "RDS",
			Category: "Database",
			Names: []string{
				"prod-billing-db",
				"prod-configs-vpc-database",
				"prod-fluxy-vpc-database",
				"prod-notification-configs-vpc-database",
				"prod-users-vpc-database",
			},
		},
	}, resp)
}

func TestToSnakeCase(t *testing.T) {
	assert.Equal(t, "dbinstance_identifier", toSnakeCase("DBInstanceIdentifier"))
	assert.Equal(t, "queue_name", toSnakeCase("QueueName"))
	assert.Equal(t, "load_balancer_name", toSnakeCase("LoadBalancerName"))
	assert.Equal(t, "function_name", toSnakeCase("FunctionName"))
}

func TestTypesToLabelNames(t *testing.T) {
	assert.Equal(t, map[string]model.LabelName{
		"RDS":    model.LabelName("dbinstance_identifier"),
		"SQS":    model.LabelName("queue_name"),
		"ELB":    model.LabelName("load_balancer_name"),
		"Lambda": model.LabelName("function_name"),
	}, typesToLabelNames)
}

func TestTypesToCategories(t *testing.T) {
	assert.Equal(t, map[string]string{
		"RDS":    "Database",
		"SQS":    "Queue",
		"ELB":    "Load Balancer",
		"Lambda": "λ-Function",
	}, categories)
}
