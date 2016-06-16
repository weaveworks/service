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

type serverHandlerFunc func(w http.ResponseWriter, r *http.Request, orgName string)

const (
	orgName   = "%21?ЖЗИЙ%2FК%$?"
	orgID     = "somePersistentInternalID"
	orgToken  = "token123"
	orgCookie = "cookie123"
)

func dummyServer() *httptest.Server {
	serverHandler := mux.NewRouter()
	serverHandler.Methods("GET").Path("/private/api/users/lookup/{orgName}").
		HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			localOrgName := mux.Vars(r)["orgName"]
			localOrgCookie, err := r.Cookie(client.AuthCookieName)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if localOrgName == orgName && orgCookie == localOrgCookie.Value {
				w.WriteHeader(http.StatusOK)
				fmt.Fprintf(w, `{ "organizationID": "%s" }`, orgID)
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
		res, err := auth.AuthenticateOrg(req, orgName)
		assert.NoError(t, err, "Unexpected error from authenticator")
		assert.Equal(t, orgID, res, "Unexpected organization")
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
		res, err := auth.AuthenticateOrg(req, orgName)
		assert.Error(t, err, "Unexpected successful authentication")
		assert.Equal(t, "", res, "Unexpected organization")
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
			OrgName:       func(*http.Request) (string, bool) { return orgName, true },
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
			OrgName:       func(*http.Request) (string, bool) { return orgName, true },
			OutputHeader:  headerName,
		}
		result := testMiddleware(mw, req)
		assert.Equal(t, result.Code, http.StatusUnauthorized, "")
		assert.Equal(t, result.Body.Bytes(), []byte(nil), "")
	}
}
