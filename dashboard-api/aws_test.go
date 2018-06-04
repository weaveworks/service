package main

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/weaveworks/service/dashboard-api/aws"

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
			Type:     string(aws.RDS.Type),
			Category: aws.Database,
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
