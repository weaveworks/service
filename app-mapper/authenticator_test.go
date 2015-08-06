package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testAuthenticator(t *testing.T, serverHandler http.Handler, testFunc func(a authenticator)) {
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
	expectedOrganizationID := "foo"
	serverHandler := mux.NewRouter()
	serverHandler.HandleFunc("/private/api/users/lookup/{org}", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method, "Unexpected method")
		assert.Equal(t, mux.Vars(r)["org"], expectedOrganizationID)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{ "organizationID": "` + expectedOrganizationID + `" }`))
	})

	testFunc := func(a authenticator) {
		r := newRequestToAuthenticate(t, "someCookieValue", "")
		res, err := a.authenticate(r, expectedOrganizationID)
		assert.NoError(t, err, "Unexpected error from authenticator")
		assert.Equal(t, expectedOrganizationID, res.OrganizationID, "Unexpected organization")

		r = newRequestToAuthenticate(t, "", "someAuthHeaderValue")
		res, err = a.authenticate(r, expectedOrganizationID)
		assert.NoError(t, err, "Unexpected error from authenticator")
		assert.Equal(t, expectedOrganizationID, res.OrganizationID, "Unexpected organization")
	}

	testAuthenticator(t, serverHandler, testFunc)
}

func TestDenyAccess(t *testing.T) {
	serverHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})

	testFunc := func(a authenticator) {
		r := newRequestToAuthenticate(t, "someCookieValue", "")
		_, err := a.authenticate(r, "whocares")
		assert.Error(t, err, "Unexpected successful authentication")
	}

	testAuthenticator(t, serverHandler, testFunc)
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

	serverHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(authCookieName)
		if err == nil {
			obtainedAuthCookieValue = cookie.Value
		}
		obtainedAuthHeaderValue = r.Header.Get(authHeaderName)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{ "organizationID": "foo" }`))
	})

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
			_, err := a.authenticate(r, "foo")
			assert.NoError(t, err, "Unexpected error from authenticator")
			assert.Equal(t, input.cookie, obtainedAuthCookieValue)
			assert.Equal(t, input.header, obtainedAuthHeaderValue)
		}

	}

	testAuthenticator(t, serverHandler, testFunc)
}

func TestBadServerResponse(t *testing.T) {
	var responseBody []byte
	serverHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(responseBody)
	})

	testFunc := func(a authenticator) {
		for _, badResponse := range []string{
			``,
			`{}`,
			`{ "SomeBogusField": "foo" }`,
			` garbager osaij oasi98 llk;fs `,
		} {
			responseBody = []byte(badResponse)
			r := newRequestToAuthenticate(t, "someCookieValue", "")
			_, err := a.authenticate(r, "foo")
			assert.Error(t, err, "Unexpected successful request")
		}

	}

	testAuthenticator(t, serverHandler, testFunc)
}
