package client_test

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/weaveworks/common/middleware"
	"github.com/weaveworks/service/users/client"
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

func assertResponse(t *testing.T, m middleware.Interface, req *http.Request, err error, status int, body string) {
	assert.NoError(t, err)
	rec := httptest.NewRecorder()
	m.Wrap(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {})).ServeHTTP(rec, req)
	assert.Equal(t, status, rec.Code)
	bodyBytes, err := ioutil.ReadAll(rec.Body)
	assert.NoError(t, err)
	assert.Equal(t, body, string(bodyBytes))
}
