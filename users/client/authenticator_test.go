package client_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/weaveworks/scope/common/middleware"

	"github.com/weaveworks/service/users/client"
)

const (
	orgExternalID = "%21?ЖЗИЙ%2FК%$?"
	orgID         = "somePersistentInternalID"
	orgToken      = "Scope-Probe token=token123"
	orgCookie     = "cookie123"
	userID        = "user12346"
)

func dummyServer() *httptest.Server {
	serverHandler := mux.NewRouter()
	serverHandler.Methods("GET").Path("/private/api/users/lookup/{orgExternalID}").
		HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			localOrgExternalID := mux.Vars(r)["orgExternalID"]
			localOrgCookie, err := r.Cookie(client.AuthCookieName)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if localOrgExternalID == orgExternalID && orgCookie == localOrgCookie.Value {
				w.WriteHeader(http.StatusOK)
				fmt.Fprintf(w, `{ "organizationID": "%s", "userID": "%s" }`, orgID, userID)
				return
			}
			w.WriteHeader(http.StatusUnauthorized)
			return
		})

	serverHandler.Methods("GET").Path("/private/api/users/lookup").
		HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			localAuthToken := r.Header.Get(client.AuthHeaderName)
			if orgToken == localAuthToken {
				w.WriteHeader(http.StatusOK)
				fmt.Fprintf(w, `{ "organizationID": "%s" }`, orgID)
				return
			}
			w.WriteHeader(http.StatusUnauthorized)
			return
		})
	return httptest.NewServer(serverHandler)
}

func TestAuth(t *testing.T) {
	server := dummyServer()
	defer server.Close()
	auth := client.MakeAuthenticator("web", server.URL, client.AuthenticatorOptions{})

	// Test we can auth based on the headers
	{
		req, _ := http.NewRequest("GET", "http://example.com/request?arg1=foo&arg2=bar", nil)
		req.Header.Set(client.AuthHeaderName, orgToken)
		res, err := auth.AuthenticateProbe(req)
		assert.NoError(t, err, "Unexpected error from authenticator")
		assert.Equal(t, orgID, res, "Unexpected organization")
	}

	// Test we can auth based on cookies
	{
		req, _ := http.NewRequest("GET", "http://example.com/request?arg1=foo&arg2=bar", nil)
		req.AddCookie(&http.Cookie{
			Name:  client.AuthCookieName,
			Value: orgCookie,
		})
		res, user, err := auth.AuthenticateOrg(req, orgExternalID)
		assert.NoError(t, err, "Unexpected error from authenticator")
		assert.Equal(t, orgID, res, "Unexpected organization")
		assert.Equal(t, userID, user, "Unexpected user")
	}

	// Test denying access for headers
	{
		req, _ := http.NewRequest("GET", "http://example.com/request?arg1=foo&arg2=bar", nil)
		req.Header.Set(client.AuthHeaderName, "This is not the right value")
		res, err := auth.AuthenticateProbe(req)
		assert.Error(t, err, "Unexpected successful authentication")
		assert.Equal(t, "", res, "Unexpected organization")
	}

	// Test denying access for cookies
	{
		req, _ := http.NewRequest("GET", "http://example.com/request?arg1=foo&arg2=bar", nil)
		req.AddCookie(&http.Cookie{
			Name:  client.AuthCookieName,
			Value: "Not the right cookie",
		})
		res, user, err := auth.AuthenticateOrg(req, orgExternalID)
		assert.Error(t, err, "Unexpected successful authentication")
		assert.Equal(t, "", res, "Unexpected organization")
		assert.Equal(t, "", user, "Unexpected user")
	}
}

func TestMiddleware(t *testing.T) {
	server := dummyServer()
	defer server.Close()
	auth := client.MakeAuthenticator("web", server.URL, client.AuthenticatorOptions{})

	var (
		body       = []byte("OK")
		headerName = "X-Some-Header"
	)

	testMiddleware := func(mw middleware.Interface, req *http.Request) *httptest.ResponseRecorder {
		wrapper := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, r.Header.Get(headerName), orgID, "")
			w.WriteHeader(http.StatusOK)
			w.Write(body)
		}))
		recorder := httptest.NewRecorder()
		wrapper.ServeHTTP(recorder, req)
		return recorder
	}

	// Test we can auth based on the headers
	{
		req, _ := http.NewRequest("GET", "http://example.com/request?arg1=foo&arg2=bar", nil)
		req.Header.Set(client.AuthHeaderName, orgToken)
		mw := client.AuthProbeMiddleware{
			Authenticator: auth,
			OutputHeader:  headerName,
		}
		result := testMiddleware(mw, req)
		assert.Equal(t, result.Code, http.StatusOK, "")
		assert.Equal(t, result.Body.Bytes(), body, "")
	}

	// Test we can auth based on cookies
	{
		req, _ := http.NewRequest("GET", "http://example.com/request?arg1=foo&arg2=bar", nil)
		req.AddCookie(&http.Cookie{
			Name:  client.AuthCookieName,
			Value: orgCookie,
		})
		mw := client.AuthOrgMiddleware{
			Authenticator: auth,
			OrgExternalID: func(*http.Request) (string, bool) { return orgExternalID, true },
			OutputHeader:  headerName,
		}
		result := testMiddleware(mw, req)
		assert.Equal(t, result.Code, http.StatusOK, "")
		assert.Equal(t, result.Body.Bytes(), body, "")
	}

	// Test denying access for headers
	{
		req, _ := http.NewRequest("GET", "http://example.com/request?arg1=foo&arg2=bar", nil)
		req.Header.Set(client.AuthHeaderName, "This is not the right value")
		mw := client.AuthProbeMiddleware{
			Authenticator: auth,
			OutputHeader:  headerName,
		}
		result := testMiddleware(mw, req)
		assert.Equal(t, result.Code, http.StatusUnauthorized, "")
		assert.Equal(t, result.Body.Bytes(), []byte(nil), "")
	}

	// Test denying access for cookies
	{
		req, _ := http.NewRequest("GET", "http://example.com/request?arg1=foo&arg2=bar", nil)
		req.AddCookie(&http.Cookie{
			Name:  client.AuthCookieName,
			Value: "Not the right cookie",
		})
		mw := client.AuthOrgMiddleware{
			Authenticator: auth,
			OrgExternalID: func(*http.Request) (string, bool) { return orgExternalID, true },
			OutputHeader:  headerName,
		}
		result := testMiddleware(mw, req)
		assert.Equal(t, result.Code, http.StatusUnauthorized, "")
		assert.Equal(t, result.Body.Bytes(), []byte(nil), "")
	}
}
