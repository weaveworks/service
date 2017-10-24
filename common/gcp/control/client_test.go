package control_test

import (
	"flag"
	"testing"

	"github.com/stretchr/testify/assert"

	"context"
	"github.com/weaveworks/service/common/gcp/control"
	"google.golang.org/api/servicecontrol/v1"
	"gopkg.in/h2non/gock.v1"
)

const (
	basePath    = "https://servicecontrol.googleapis.com"
	serviceName = "weaveworks-test-cloudmarketplacepartner.googleapis.com"
)

var config control.Config

func init() {
	config.RegisterFlags(flag.CommandLine)
	config.ServiceAccountKeyFile = "../../../testdata/google-service-account-key.json"
	flag.Parse()
}

func TestClient_OperationID(t *testing.T) {
	cl, err := control.NewClient(config)
	assert.NoError(t, err)

	first := cl.OperationID("foo")
	assert.Equal(t, "aa752cea-8222-5bc8-acd9-555b090c0ccb", first)

	second := cl.OperationID("foo")
	assert.Equal(t, first, second)

	third := cl.OperationID("fooz")
	assert.NotEqual(t, first, third)
}

func TestClient_Report(t *testing.T) {
	defer gock.Off()

	mockOauth()
	gock.New(basePath).
		Post("/v1/services/" + serviceName + ":report").
		Reply(200).
		BodyString(`{"serviceConfigId": "something"}`)

	cl, err := control.NewClient(config)
	assert.NoError(t, err)

	ops := []*servicecontrol.Operation{}
	err = cl.Report(context.Background(), serviceName, ops)
	assert.NoError(t, err)
}

func TestClient_ReportError(t *testing.T) {
	defer gock.Off()

	mockOauth()
	gock.New(basePath).
		Post("/v1/services/" + serviceName + ":report").
		Reply(200).
		JSON(map[string]interface{}{
			"reportErrors": []*servicecontrol.ReportError{{
				OperationId: "foo123",
				Status: &servicecontrol.Status{
					Code:    987,
					Message: "Hello there, something went wrong.",
				},
			}},
		})

	cl, err := control.NewClient(config)
	assert.NoError(t, err)

	ops := []*servicecontrol.Operation{}
	err = cl.Report(context.Background(), serviceName, ops)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "foo123")
	assert.Contains(t, err.Error(), "987")
	assert.Contains(t, err.Error(), "Hello there, something went wrong.")
}

// mockOauth mocks the oauth2 token request
func mockOauth() {
	gock.New("https://accounts.google.com").
		Post("/o/oauth2/token").
		Reply(200).
		JSON(map[string]interface{}{
			"access_token":  "ya29.Foo",
			"token_type":    "",
			"expires_in":    0,
			"refresh_token": "",
		})
}