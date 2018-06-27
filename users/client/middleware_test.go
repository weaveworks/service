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

func TestGCPLoginSecretMiddleware_ValidRequest(t *testing.T) {
	m := client.GCPLoginSecretMiddleware{Secret: "s3cr3t"}
	assert.Equal(t, "cb26769c6c76965ed9eebb52fb08362e974e7724", m.Tokenise("E-53D7-7F2A-1D98-F3F5", "1510582292912"))

	req, err := http.NewRequest("GET", "https://weave.test/login/gcp/E-53D7-7F2A-1D98-F3F5?timestamp=1510582292912&ssoToken=cb26769c6c76965ed9eebb52fb08362e974e7724", nil)
	assertResponse(t, m, req, err, http.StatusOK, "")
}

func TestGCPLoginSecretMiddleware_InvalidRequests(t *testing.T) {
	m := client.GCPLoginSecretMiddleware{Secret: "s3cr3t"}

	// Invalid timestamp: empty
	req, err := http.NewRequest("GET", "https://weave.test/login/gcp/E-53D7-7F2A-1D98-F3F5?ssoToken=cb26769c6c76965ed9eebb52fb08362e974e7724", nil)
	assertResponse(t, m, req, err, http.StatusBadRequest, "Bad Request\n") // No detail on the error is provided, for security reasons.

	// Invalid timestamp: not a numeric
	req, err = http.NewRequest("GET", "https://weave.test/login/gcp/E-53D7-7F2A-1D98-F3F5?timestamp=foo&ssoToken=cb26769c6c76965ed9eebb52fb08362e974e7724", nil)
	assertResponse(t, m, req, err, http.StatusBadRequest, "Bad Request\n") // No detail on the error is provided, for security reasons.

	// Invalid timestamp: in seconds instead of milliseconds
	req, err = http.NewRequest("GET", "https://weave.test/login/gcp/E-53D7-7F2A-1D98-F3F5?timestamp=1510582292&ssoToken=cb26769c6c76965ed9eebb52fb08362e974e7724", nil)
	assertResponse(t, m, req, err, http.StatusBadRequest, "Bad Request\n") // No detail on the error is provided, for security reasons.

	// Invalid token: malformed
	req, err = http.NewRequest("GET", "https://weave.test/login/gcp/E-53D7-7F2A-1D98-F3F5?timestamp=1510582292912&ssoToken=fooooooooooooooooooooooooooooooooooooooo", nil)
	assertResponse(t, m, req, err, http.StatusBadRequest, "Bad Request\n") // No detail on the error is provided, for security reasons.

	// Invalid GCP account ID: malformed
	req, err = http.NewRequest("GET", "https://weave.test/login/gcp/Z-ZZZZ-ZZZZ-ZZZZ-ZZZZ?timestamp=1510582292913&ssoToken=cb26769c6c76965ed9eebb52fb08362e974e7724", nil)
	assertResponse(t, m, req, err, http.StatusBadRequest, "Bad Request\n") // No detail on the error is provided, for security reasons.

	// Invalid request: token was not formed with the same timestamp provided in request
	req, err = http.NewRequest("GET", "https://weave.test/login/gcp/E-53D7-7F2A-1D98-F3F5?timestamp=1510582292913&ssoToken=cb26769c6c76965ed9eebb52fb08362e974e7724", nil)
	assertResponse(t, m, req, err, http.StatusUnauthorized, "Unauthorized\n") // No detail on the error is provided, for security reasons.

	// Invalid request: token was not formed with the same keyForSsoLogin provided in request
	req, err = http.NewRequest("GET", "https://weave.test/login/gcp/A-0000-0000-0000-0000?timestamp=1510582292912&ssoToken=cb26769c6c76965ed9eebb52fb08362e974e7724", nil)
	assertResponse(t, m, req, err, http.StatusUnauthorized, "Unauthorized\n") // No detail on the error is provided, for security reasons.

	// Invalid request: token was not formed with the same secret as the server
	m = client.GCPLoginSecretMiddleware{Secret: "An0ther_s3cr3t"}
	req, err = http.NewRequest("GET", "https://weave.test/login/gcp/E-53D7-7F2A-1D98-F3F5?timestamp=1510582292912&ssoToken=cb26769c6c76965ed9eebb52fb08362e974e7724", nil)
	assertResponse(t, m, req, err, http.StatusUnauthorized, "Unauthorized\n") // No detail on the error is provided, for security reasons.
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
						IntegrationType:  "github",
						SecretID:         "sercret-abc",
						SecretSigningKey: "",
						CreatedAt:        time.Now(),
					},
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
						IntegrationType:  "github",
						SecretID:         "sercret-abc",
						SecretSigningKey: "signing-key-123",
						CreatedAt:        time.Now(),
					},
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
						IntegrationType:  "github",
						SecretID:         "sercret-abc",
						SecretSigningKey: "signing-key-123",
						CreatedAt:        time.Now(),
					},
				}, nil)
		req.Header.Set("X-Hub-Signature", genGithubMAC([]byte("payload"), []byte("signing-key-invalid")))
		assertResponse(t, m, req, err, http.StatusBadGateway, "")
	}

	{ // Webhook does not exist
		u.EXPECT().
			LookupOrganizationWebhookUsingSecretID(gomock.Any(), &users.LookupOrganizationWebhookUsingSecretIDRequest{
				SecretID: "secret-invalid",
			}).
			Return(nil, httpgrpc.Errorf(http.StatusUnauthorized, ""))

		req, err := http.NewRequest("GET", "https://weave.test/webhooks/secret-invalid", nil)
		req = mux.SetURLVars(req, map[string]string{"secretID": "secret-invalid"})
		assertResponse(t, m, req, err, http.StatusUnauthorized, "Unauthorized\n")
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
