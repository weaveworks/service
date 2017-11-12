package partner_test

import (
	"context"
	"encoding/json"
	"flag"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gopkg.in/h2non/gock.v1"

	"github.com/weaveworks/service/common/gcp/partner"
)

const (
	pendingName = "partnerSubscriptions/47426f1a-d744-4249-ae84-3f4fe194c107"
	pending     = `
    {
      "name": "partnerSubscriptions/47426f1a-d744-4249-ae84-3f4fe194c107",
      "externalAccountId": "E-F65F-C51C-67FE-D42F",
      "version": "1508480169982224",
      "status": "PENDING",
      "subscribedResources": [
        {
          "subscriptionProvider": "weaveworks-public-cloudmarketplacepartner.googleapis.com",
          "resource": "weave-cloud",
          "labels": {
            "weaveworks-public-cloudmarketplacepartner.googleapis.com/ServiceLevel": "standard"
          }
        }
      ],
      "startDate": {
        "year": 2017,
        "month": 10,
        "day": 19
      },
      "createTime": "2017-10-20T06:16:09.982224Z",
      "updateTime": "2017-10-20T06:16:09.982224Z",
      "requiredApprovals": [
        {
          "name": "default-approval",
          "status": "PENDING"
        }
      ]
    }
`
	list = `{"subscriptions": [` + pending + `]}`

	basePath = "https://cloudbilling.googleapis.com"
)

var config partner.Config

func init() {
	config.RegisterFlags(flag.CommandLine)
	config.ServiceAccountKeyFile = "../../../testdata/google-service-account-key.json"
	flag.Parse()
}

// Unmarshal then marshal needs to lead to the same json.
func TestSubscription_Marshal(t *testing.T) {
	sub := &partner.Subscription{}
	err := json.Unmarshal([]byte(pending), sub)
	assert.NoError(t, err)

	out, err := json.Marshal(sub)
	assert.JSONEq(t, pending, string(out))
}

func TestClient_Approve(t *testing.T) {
	defer gock.Off()

	mockOauth()
	gock.New(basePath).
		Post("/v1/" + pendingName + ":approve").
		Reply(200).BodyString(pending)

	cl := createClient(t)
	sub, err := cl.ApproveSubscription(context.Background(), pendingName, nil)
	assert.NoError(t, err)

	assert.Equal(t, pendingName, sub.Name)
}

func TestClient_Deny(t *testing.T) {
	defer gock.Off()

	mockOauth()
	gock.New(basePath).
		Post("/v1/" + pendingName + ":deny").
		Reply(200).BodyString(pending)

	cl := createClient(t)
	sub, err := cl.DenySubscription(context.Background(), pendingName, nil)
	assert.NoError(t, err)

	assert.Equal(t, pendingName, sub.Name)
}

func TestClient_Get(t *testing.T) {
	defer gock.Off()

	mockOauth()
	gock.New(basePath).
		Get("/v1/" + pendingName).
		Reply(200).BodyString(pending)

	cl := createClient(t)
	sub, err := cl.GetSubscription(context.Background(), pendingName)
	assert.NoError(t, err)

	assert.Equal(t, pendingName, sub.Name)
	assert.Equal(t, "E-F65F-C51C-67FE-D42F", sub.ExternalAccountID)
	assert.Equal(t, partner.Pending, sub.Status)
	assert.True(t, sub.StartDate.Time(time.UTC).Equal(time.Date(2017, 10, 19, 0, 0, 0, 0, time.UTC)))
}

func TestClient_List(t *testing.T) {
	defer gock.Off()

	mockOauth()
	gock.New(basePath).
		Get("/v1/partnerSubscriptions").
		Reply(200).BodyString(list)

	cl := createClient(t)
	sub, err := cl.ListSubscriptions(context.Background(), "foo")
	assert.NoError(t, err)

	assert.Len(t, sub, 1)
}

func TestClient_GetError(t *testing.T) {
	defer gock.Off()

	mockOauth()
	gock.New(basePath).
		Get("/v1/" + pendingName).
		Reply(400).JSON(map[string]interface{}{
		"code":    400,
		"message": "Something something",
		"status":  "INVALID_ARGUMENT",
	})

	cl := createClient(t)
	sub, err := cl.GetSubscription(context.Background(), pendingName)
	assert.Nil(t, sub)
	assert.Error(t, err)
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

func createClient(t *testing.T) *partner.Client {
	cl, err := partner.NewClient(config)
	assert.NoError(t, err)
	return cl
}
