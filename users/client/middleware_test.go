package client_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

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
