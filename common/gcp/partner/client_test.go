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

	apiURI = "https://weave.test"
)

// Mocked service JSON account key, the private key was explicitly generated for this test.
var jsonKey = map[string]string{
	"type":                        "service_account",
	"project_id":                  "weaveworks-test",
	"private_key_id":              "abc890",
	"private_key":                 "-----BEGIN RSA PRIVATE KEY-----\nMIICXQIBAAKBgQC7sY2DwxQHWs99Fs1u4aMWNzQZGr29v8gbQTFTaesjtY4h1Z1j\nBYjprYzEg9f+jLg1d4P7cSQfPmCVnSmIwDEi2Ih7RCwf/nbfdLSfWrtw24eBXNyL\nQMH63Q3SlGK8zRcbjhbQt283Gg46f6y0Lt04lcakxiBqhMg44bzPdb1cZQIDAQAB\nAoGALjZhKXf2jnkFbT8YBZz4kpe09BlpbjayBkPe6TLC+l/RRvNZdO//7ckVR61O\nmRX8pO1wSZBp3Gd3UF8JwunPLuEjLSpKzxINlKgVTz/DFrYzC4uwgqBZ0rwHKsL7\nMzndzf3M+blZLBspAsCONMJviVZZYr7xzxixe03ML5n1m2ECQQDlgHhN6iEHhlrJ\nQta4wWQ8vvdw03d6gVMead5+ZRgV9TzvU6yYmbhHpBjg7St5H5xEWRhwaNKreNwN\nnPMnCSr3AkEA0V1UW8uOj+7DBmpu0al6g3epi6NQLNx3KwA+jfaw8kLfEhYns2mn\nHTeF3OyiNf1kYR95axg2FHIz2DGrud6ggwJBAI1V2MDi9wRTUYWwi9ur/bcLRAdP\ns7zV+AI64LKmP3cGWEhrF1fDEyHLhSa/6I3nUa0l0U8ovtSq0Znwli3sD3ECQHy+\n+EmtsucN44RKHHeuXMJCpXH/QAFK53Jmtd8OkwX2VEXJj6Q2Go2tDITDNi+nKI06\nHLVz+p0aIsv5ZJHeFZMCQQC2cunzP6wslKo/JMbvh5Vw5Xj/TNRi+Xp2ucvkh4Ix\n12rMVEoSFh/oQpHJE7aWlOsA+ovCn/WiIjv3A3lmxDOq\n-----END RSA PRIVATE KEY-----\n",
	"client_email":                "cloud-launcher@weaveworks-public.iam.gserviceaccount.com",
	"client_id":                   "123456",
	"auth_apiURI":                 "https://accounts.google.com/o/oauth2/auth",
	"token_apiURI":                "https://accounts.google.com/o/oauth2/token",
	"auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
	"client_x509_cert_url":        "https://www.googleapis.com/robot/v1/metadata/x509/cloud-launcher%40weaveworks-public.iam.gserviceaccount.com",
}

var config partner.Config

func init() {
	config.RegisterFlags(flag.CommandLine)
	flag.Parse()
	config.URL = apiURI + "/v1"
}

// Unmarshal then marshal needs to lead to the same json.
func TestSubscription_Marshal(t *testing.T) {
	sub := &partner.Subscription{}
	err := json.Unmarshal([]byte(pending), sub)
	assert.NoError(t, err)

	out, err := json.Marshal(sub)
	assert.JSONEq(t, pending, string(out))
}

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
	bs, err := json.Marshal(jsonKey)
	assert.NoError(t, err)
	cl, err := partner.NewClientFromJSON(bs, config)
	assert.NoError(t, err)
	return cl
}

func TestClient_Approve(t *testing.T) {
	defer gock.Off()

	mockOauth()
	gock.New(apiURI).
		Post("/v1/" + pendingName + ":approve").
		Reply(200).BodyString(pending)

	cl := createClient(t)
	sub, err := cl.ApproveSubscription(context.Background(), pendingName)
	assert.NoError(t, err)

	assert.Equal(t, pendingName, sub.Name)
}

func TestClient_Deny(t *testing.T) {
	defer gock.Off()

	mockOauth()
	gock.New(apiURI).
		Post("/v1/" + pendingName + ":deny").
		Reply(200).BodyString(pending)

	cl := createClient(t)
	sub, err := cl.DenySubscription(context.Background(), pendingName)
	assert.NoError(t, err)

	assert.Equal(t, pendingName, sub.Name)
}

func TestClient_Get(t *testing.T) {
	defer gock.Off()

	mockOauth()
	gock.New(apiURI).
		Get("/v1/" + pendingName).
		Reply(200).BodyString(pending)

	cl := createClient(t)
	sub, err := cl.GetSubscription(context.Background(), pendingName)
	assert.NoError(t, err)

	assert.Equal(t, pendingName, sub.Name)
	assert.Equal(t, "E-F65F-C51C-67FE-D42F", sub.ExternalAccountID)
	assert.Equal(t, partner.StatusPending, sub.Status)
	assert.True(t, sub.StartDate.Time(time.UTC).Equal(time.Date(2017, 10, 19, 0, 0, 0, 0, time.UTC)))
}

func TestClient_List(t *testing.T) {
	defer gock.Off()

	mockOauth()
	gock.New(apiURI).
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
	gock.New(apiURI).
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
