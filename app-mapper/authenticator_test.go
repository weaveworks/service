package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type serverHandlerFunc func(w http.ResponseWriter, r *http.Request)

func testAuthenticator(t *testing.T, shFunc serverHandlerFunc, testFunc func(a authenticator)) {
	serverHandler := mux.NewRouter()
	serverHandler.HandleFunc("/private/api/users/lookup", shFunc)
	authenticatorServer := httptest.NewServer(serverHandler)
	defer authenticatorServer.Close()
	parsedAuthenticatorURL, err := url.Parse(authenticatorServer.URL)
	require.NoError(t, err, "Cannot parse authenticatorServer URL")
	testFunc(&webAuthenticator{parsedAuthenticatorURL.Host})
}

func newRequestToAuthenticate(t *testing.T, authCookieValue string, authHeaderValue string) *http.Request {
	req, err := http.NewRequest("GET", "http://example.com/request?arg1=foo&arg2=bar", nil)
	require.NoError(t, err, "Cannot create request")
	if len(authCookieValue) > 0 {
		c := http.Cookie{
			Name:  authCookieName,
			Value: authCookieValue,
		}
		req.AddCookie(&c)
	}
	if len(authHeaderValue) > 0 {
		req.Header.Set(authHeaderName, authHeaderValue)
	}
	return req
}

func TestAuthorize(t *testing.T) {
	const organizationID = "somePersistentInternalID"
	shFunc := func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method, "Unexpected method")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{ "organizationID": "%s" }`, organizationID)
	}

	testFunc := func(a authenticator) {
		r := newRequestToAuthenticate(t, "someCookieValue", "")
		res, err := a.authenticate(r)
		assert.NoError(t, err, "Unexpected error from authenticator")
		assert.Equal(t, organizationID, res.OrganizationID, "Unexpected organization")

		r = newRequestToAuthenticate(t, "", "someAuthHeaderValue")
		res, err = a.authenticate(r)
		assert.NoError(t, err, "Unexpected error from authenticator")
		assert.Equal(t, organizationID, res.OrganizationID, "Unexpected organization")
	}

	testAuthenticator(t, shFunc, testFunc)
}

func TestDenyAccess(t *testing.T) {
	shFunc := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}

	testFunc := func(a authenticator) {
		r := newRequestToAuthenticate(t, "someCookieValue", "")
		_, err := a.authenticate(r)
		assert.Error(t, err, "Unexpected successful authentication")
	}

	testAuthenticator(t, shFunc, testFunc)
}

func TestCredentialForwarding(t *testing.T) {
	const (
		authCookieValue = "someCookieValue"
		authHeaderValue = "someAuthHeaderValue"
	)
	var (
		obtainedAuthCookieValue string
		obtainedAuthHeaderValue string
	)

	shFunc := func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(authCookieName)
		if err == nil {
			obtainedAuthCookieValue = cookie.Value
		}
		obtainedAuthHeaderValue = r.Header.Get(authHeaderName)
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{ "organizationID": "somePersistentInternalID" }`)
	}

	testFunc := func(a authenticator) {

		for _, input := range []struct {
			cookie, header string
		}{
			{authCookieValue, ""},
			{"", authHeaderValue},
			{authCookieValue, authHeaderValue},
		} {
			obtainedAuthCookieValue = ""
			obtainedAuthHeaderValue = ""
			r := newRequestToAuthenticate(t, input.cookie, input.header)
			_, err := a.authenticate(r)
			assert.NoError(t, err, "Unexpected error from authenticator")
			assert.Equal(t, input.cookie, obtainedAuthCookieValue)
			assert.Equal(t, input.header, obtainedAuthHeaderValue)
		}

	}

	testAuthenticator(t, shFunc, testFunc)
}

func TestBadServerResponse(t *testing.T) {
	var responseBody []byte

	shFunc := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(responseBody)
	}

	testFunc := func(a authenticator) {
		for _, badResponse := range []string{
			``,
			`{}`,
			`{ "SomeBogusField": "foo" }`,
			` garbager osaij oasi98 llk;fs `,
		} {
			responseBody = []byte(badResponse)
			r := newRequestToAuthenticate(t, "someCookieValue", "")
			_, err := a.authenticate(r)
			assert.Error(t, err, "Unexpected successful request")
		}

	}

	testAuthenticator(t, shFunc, testFunc)
}
