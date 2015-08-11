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

type serverHandlerFunc func(w http.ResponseWriter, r *http.Request, orgName string)

func testAuthenticator(t *testing.T, shFunc serverHandlerFunc, testFunc func(a authenticator)) {
	serverHandler := mux.NewRouter()
	serverHandler.HandleFunc("/private/api/users/lookup/{orgName}", func(w http.ResponseWriter, r *http.Request) {
		shFunc(w, r, mux.Vars(r)["orgName"])
	})
	serverHandler.HandleFunc("/private/api/users/lookup", func(w http.ResponseWriter, r *http.Request) {
		shFunc(w, r, "")
	})
	authenticatorServer := httptest.NewServer(serverHandler)
	defer authenticatorServer.Close()
	parsedAuthenticatorURL, err := url.Parse(authenticatorServer.URL)
	require.NoError(t, err, "Cannot parse authenticatorServer URL")
	testFunc(&webAuthenticator{parsedAuthenticatorURL.Host})
}

func authenticateOrg(t *testing.T, a authenticator, orgName string) (authenticatorResponse, error) {
	req, err := http.NewRequest("GET", "http://example.com/request?arg1=foo&arg2=bar", nil)
	require.NoError(t, err, "Cannot create request")
	c := http.Cookie{
		Name:  authCookieName,
		Value: "someAuthCookieValue",
	}
	req.AddCookie(&c)
	return a.authenticateOrg(req, orgName)
}

func authenticateProbe(t *testing.T, a authenticator) (authenticatorResponse, error) {
	req, err := http.NewRequest("GET", "http://example.com/request?arg1=foo&arg2=bar", nil)
	require.NoError(t, err, "Cannot create request")
	req.Header.Set(authHeaderName, "someAuthHeaderValue")
	return a.authenticateProbe(req)
}

func TestAuthorizeOrg(t *testing.T) {
	const organizationID = "somePersistentInternalID"
	shFunc := func(w http.ResponseWriter, r *http.Request, orgName string) {
		assert.Equal(t, "GET", r.Method, "Unexpected method")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{ "organizationID": "%s" }`, organizationID)
	}

	testFunc := func(a authenticator) {
		const organizationName = "somePublicOrgName"

		res, err := authenticateOrg(t, a, organizationName)
		assert.NoError(t, err, "Unexpected error from authenticator")
		assert.Equal(t, organizationID, res.OrganizationID, "Unexpected organization")

		res, err = authenticateProbe(t, a)
		assert.NoError(t, err, "Unexpected error from authenticator")
		assert.Equal(t, organizationID, res.OrganizationID, "Unexpected organization")
	}

	testAuthenticator(t, shFunc, testFunc)
}

func TestEncoding(t *testing.T) {
	var recordedOrganizationName string
	shFunc := func(w http.ResponseWriter, r *http.Request, orgName string) {
		recordedOrganizationName = orgName
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{ "organizationID": "somePersistentInternalID" }`)
	}

	testFunc := func(a authenticator) {
		const organizationName = "%21?ЖЗИЙ%2FК%$?"

		_, err := authenticateOrg(t, a, organizationName)
		assert.NoError(t, err, "Unexpected error from authenticator")
		assert.Equal(t, organizationName, recordedOrganizationName, "Unexpected organization")
	}

	testAuthenticator(t, shFunc, testFunc)
}

func TestDenyAccess(t *testing.T) {
	shFunc := func(w http.ResponseWriter, r *http.Request, orgID string) {
		w.WriteHeader(http.StatusUnauthorized)
	}

	testFunc := func(a authenticator) {
		_, err := authenticateOrg(t, a, "somePublicOrgName")
		assert.Error(t, err, "Unexpected successful authentication")

		_, err = authenticateProbe(t, a)
		assert.Error(t, err, "Unexpected successful authentication")
	}

	testAuthenticator(t, shFunc, testFunc)
}

func TestCredentialForwarding(t *testing.T) {
	var (
		obtainedAuthCookieValue string
		obtainedAuthHeaderValue string
	)

	shFunc := func(w http.ResponseWriter, r *http.Request, orgName string) {
		cookie, err := r.Cookie(authCookieName)
		if err == nil {
			obtainedAuthCookieValue = cookie.Value
		}
		obtainedAuthHeaderValue = r.Header.Get(authHeaderName)
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{ "organizationID": "somePersistentInternalID" }`)
	}

	testFunc := func(a authenticator) {
		_, err := authenticateOrg(t, a, "foo")
		assert.NoError(t, err, "Unexpected error from authenticator")
		assert.Equal(t, "someAuthCookieValue", obtainedAuthCookieValue)
		assert.Equal(t, "", obtainedAuthHeaderValue)

		obtainedAuthCookieValue = ""
		_, err = authenticateProbe(t, a)
		assert.NoError(t, err, "Unexpected error from authenticator")
		assert.Equal(t, "someAuthHeaderValue", obtainedAuthHeaderValue)
		assert.Equal(t, "", obtainedAuthCookieValue)
	}

	testAuthenticator(t, shFunc, testFunc)
}

func TestBadServerResponse(t *testing.T) {
	var responseBody []byte

	shFunc := func(w http.ResponseWriter, r *http.Request, orgName string) {
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
			_, err := authenticateOrg(t, a, "foo")
			assert.Error(t, err, "Unexpected successful request")

			_, err = authenticateProbe(t, a)
			assert.Error(t, err, "Unexpected successful request")
		}

	}

	testAuthenticator(t, shFunc, testFunc)
}
