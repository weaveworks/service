package client_test

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"

	"github.com/weaveworks/common/httpgrpc"
	"github.com/weaveworks/common/middleware"
	"github.com/weaveworks/service/common/constants/webhooks"
	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/client"
	"github.com/weaveworks/service/users/mock_users"
)

func TestAuthSecretMiddleware(t *testing.T) {
	mw := client.AuthSecretMiddleware{Secret: "soo"}
	req, err := http.NewRequest("GET", "https://weave.test?secret=soo", nil)
	assert.NoError(t, err)

	rec := httptest.NewRecorder()
	mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {})).
		ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestAuthSecret_Mismatch(t *testing.T) {
	mw := client.AuthSecretMiddleware{Secret: "soo"}
	req, err := http.NewRequest("GET", "https://weave.test?secret=sooz", nil)
	assert.NoError(t, err)

	rec := httptest.NewRecorder()
	mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {})).
		ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// genGithubMAC generates the GitHub HMAC signature for a message provided the secret key
func genGithubMAC(message, key []byte) string {
	mac := hmac.New(sha512.New, key)
	mac.Write(message)
	signature := mac.Sum(nil)

	hexSignature := make([]byte, hex.EncodedLen(len(signature)))
	hex.Encode(hexSignature, signature)
	return "sha512=" + string(hexSignature)
}

func TestWebhooksMiddleware(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	u := mock_users.NewMockUsersClient(ctrl)
	m := client.WebhooksMiddleware{
		UsersClient:                   u,
		WebhooksIntegrationTypeHeader: webhooks.WebhooksIntegrationTypeHeader,
	}
	now := time.Now()

	{ // Webhook exists
		u.EXPECT().
			LookupOrganizationWebhookUsingSecretID(gomock.Any(), &users.LookupOrganizationWebhookUsingSecretIDRequest{
				SecretID: "secret-abc",
			}).
			Return(
				&users.LookupOrganizationWebhookUsingSecretIDResponse{
					Webhook: &users.Webhook{
						ID:               "1",
						OrganizationID:   "100",
						IntegrationType:  "other",
						SecretID:         "secret-abc",
						SecretSigningKey: "",
						CreatedAt:        time.Now(),
					},
				}, nil)
		u.EXPECT().
			SetOrganizationWebhookFirstSeenAt(gomock.Any(), &users.SetOrganizationWebhookFirstSeenAtRequest{
				SecretID: "secret-abc",
			}).
			Return(
				&users.SetOrganizationWebhookFirstSeenAtResponse{
					FirstSeenAt: &now,
				}, nil)

		req, err := http.NewRequest("GET", "https://weave.test/webhooks/secret-abc", nil)
		req = mux.SetURLVars(req, map[string]string{"secretID": "secret-abc"})
		assertResponse(t, m, req, err, http.StatusOK, "")
	}

	{ // Webhook exists with signing key
		u.EXPECT().
			LookupOrganizationWebhookUsingSecretID(gomock.Any(), &users.LookupOrganizationWebhookUsingSecretIDRequest{
				SecretID: "secret-abc",
			}).
			Return(
				&users.LookupOrganizationWebhookUsingSecretIDResponse{
					Webhook: &users.Webhook{
						ID:               "1",
						OrganizationID:   "100",
						IntegrationType:  webhooks.GithubPushIntegrationType,
						SecretID:         "secret-abc",
						SecretSigningKey: "signing-key-123",
						CreatedAt:        time.Now(),
					},
				}, nil)
		u.EXPECT().
			SetOrganizationWebhookFirstSeenAt(gomock.Any(), &users.SetOrganizationWebhookFirstSeenAtRequest{
				SecretID: "secret-abc",
			}).
			Return(
				&users.SetOrganizationWebhookFirstSeenAtResponse{
					FirstSeenAt: &now,
				}, nil)

		req, err := http.NewRequest("GET", "https://weave.test/webhooks/secret-abc", strings.NewReader("payload"))
		req = mux.SetURLVars(req, map[string]string{"secretID": "secret-abc"})
		req.Header.Set("X-Hub-Signature", genGithubMAC([]byte("payload"), []byte("signing-key-123")))
		assertResponse(t, m, req, err, http.StatusOK, "")

		// Invalid signing key
		u.EXPECT().
			LookupOrganizationWebhookUsingSecretID(gomock.Any(), &users.LookupOrganizationWebhookUsingSecretIDRequest{
				SecretID: "secret-abc",
			}).
			Return(
				&users.LookupOrganizationWebhookUsingSecretIDResponse{
					Webhook: &users.Webhook{
						ID:               "1",
						OrganizationID:   "100",
						IntegrationType:  webhooks.GithubPushIntegrationType,
						SecretID:         "secret-abc",
						SecretSigningKey: "signing-key-123",
						CreatedAt:        time.Now(),
					},
				}, nil)
		req.Header.Set("X-Hub-Signature", genGithubMAC([]byte("payload"), []byte("signing-key-invalid")))
		assertResponse(t, m, req, err, http.StatusUnauthorized, "The GitHub signature header is invalid.\n")

	}

	{
		// Valid Gitlab shared secret
		u.EXPECT().
			LookupOrganizationWebhookUsingSecretID(gomock.Any(), &users.LookupOrganizationWebhookUsingSecretIDRequest{
				SecretID: "secret-abc",
			}).
			Return(
				&users.LookupOrganizationWebhookUsingSecretIDResponse{
					Webhook: &users.Webhook{
						ID:               "1",
						OrganizationID:   "100",
						IntegrationType:  webhooks.GitlabPushIntegrationType,
						SecretID:         "secret-abc",
						SecretSigningKey: "shared-secret-123",
						CreatedAt:        time.Now(),
					},
				}, nil)
		u.EXPECT().
			SetOrganizationWebhookFirstSeenAt(gomock.Any(), &users.SetOrganizationWebhookFirstSeenAtRequest{
				SecretID: "secret-abc",
			}).
			Return(
				&users.SetOrganizationWebhookFirstSeenAtResponse{
					FirstSeenAt: &now,
				}, nil)
		req, err := http.NewRequest("GET", "https://weave.test/webhooks/secret-abc", strings.NewReader("payload"))
		req = mux.SetURLVars(req, map[string]string{"secretID": "secret-abc"})
		req.Header.Set("X-Gitlab-Token", "shared-secret-123")
		assertResponse(t, m, req, err, http.StatusOK, "")

		// Invalid Gitlab shared secret
		u.EXPECT().
			LookupOrganizationWebhookUsingSecretID(gomock.Any(), &users.LookupOrganizationWebhookUsingSecretIDRequest{
				SecretID: "secret-abc",
			}).
			Return(
				&users.LookupOrganizationWebhookUsingSecretIDResponse{
					Webhook: &users.Webhook{
						ID:               "1",
						OrganizationID:   "100",
						IntegrationType:  webhooks.GitlabPushIntegrationType,
						SecretID:         "secret-abc",
						SecretSigningKey: "totally-different-secret-123",
						CreatedAt:        time.Now(),
					},
				}, nil)
		req.Header.Set("X-Gitlab-Token", "shared-secret-123")
		assertResponse(t, m, req, err, http.StatusUnauthorized, "The Gitlab token does not match\n")
	}

	{ // Webhook does not exist
		u.EXPECT().
			LookupOrganizationWebhookUsingSecretID(gomock.Any(), &users.LookupOrganizationWebhookUsingSecretIDRequest{
				SecretID: "secret-invalid",
			}).
			Return(nil, httpgrpc.Errorf(http.StatusNotFound, "Webhook does not exist."))

		req, err := http.NewRequest("GET", "https://weave.test/webhooks/secret-invalid", nil)
		req = mux.SetURLVars(req, map[string]string{"secretID": "secret-invalid"})
		assertResponse(t, m, req, err, http.StatusNotFound, "Webhook does not exist.\n")
	}
}

func assertResponse(t *testing.T, m middleware.Interface, req *http.Request, err error, status int, body string) {
	assert.NoError(t, err)
	rec := httptest.NewRecorder()
	m.Wrap(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {})).ServeHTTP(rec, req)
	assert.Equal(t, status, rec.Code)
	bodyBytes, err := ioutil.ReadAll(rec.Body)
	assert.NoError(t, err)
	assert.Equal(t, body, string(bodyBytes))
}
