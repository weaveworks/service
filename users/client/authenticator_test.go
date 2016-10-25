package client_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

var (
	featureFlags = []string{"a", "b"}
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
			j, _ := json.Marshal(map[string]interface{}{
				"organizationID": orgID,
				"userID":         userID,
				"featureFlags":   featureFlags,
			})
			if localOrgExternalID == orgExternalID && orgCookie == localOrgCookie.Value {
				w.WriteHeader(http.StatusOK)
				w.Write(j)
				return
			}
			w.WriteHeader(http.StatusUnauthorized)
			return
		})

	serverHandler.Methods("GET").Path("/private/api/users/lookup").
		HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			localAuthToken := r.Header.Get(client.AuthHeaderName)
			j, _ := json.Marshal(map[string]interface{}{
				"organizationID": orgID,
				"featureFlags":   featureFlags,
			})
			if orgToken == localAuthToken {
				w.WriteHeader(http.StatusOK)
				w.Write(j)
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
		w := httptest.NewRecorder()
		gotOrgID, gotFeatureFlags, err := auth.AuthenticateProbe(w, req)
		assert.NoError(t, err, "Unexpected error from authenticator")
		assert.Equal(t, orgID, gotOrgID, "Unexpected organization")
		assert.Equal(t, featureFlags, gotFeatureFlags, "Unexpected featureFlags")
	}

	// Test we can auth based on cookies
	{
		req, _ := http.NewRequest("GET", "http://example.com/request?arg1=foo&arg2=bar", nil)
		req.AddCookie(&http.Cookie{
			Name:  client.AuthCookieName,
			Value: orgCookie,
		})
		w := httptest.NewRecorder()
		gotOrgID, user, gotFeatureFlags, err := auth.AuthenticateOrg(w, req, orgExternalID)
		assert.NoError(t, err, "Unexpected error from authenticator")
		assert.Equal(t, orgID, gotOrgID, "Unexpected organization")
		assert.Equal(t, userID, user, "Unexpected user")
		assert.Equal(t, featureFlags, gotFeatureFlags, "Unexpected organization")
	}

	// Test denying access for headers
	{
		req, _ := http.NewRequest("GET", "http://example.com/request?arg1=foo&arg2=bar", nil)
		req.Header.Set(client.AuthHeaderName, "This is not the right value")
		w := httptest.NewRecorder()
		gotOrgID, gotFeatureFlags, err := auth.AuthenticateProbe(w, req)
		assert.Error(t, err, "Unexpected successful authentication")
		assert.Equal(t, "", gotOrgID, "Unexpected organization")
		assert.Len(t, gotFeatureFlags, 0, "Unexpected organization")
	}

	// Test denying access for cookies
	{
		req, _ := http.NewRequest("GET", "http://example.com/request?arg1=foo&arg2=bar", nil)
		req.AddCookie(&http.Cookie{
			Name:  client.AuthCookieName,
			Value: "Not the right cookie",
		})
		w := httptest.NewRecorder()
		gotOrgID, user, gotFeatureFlags, err := auth.AuthenticateOrg(w, req, orgExternalID)
		assert.Error(t, err, "Unexpected successful authentication")
		assert.Equal(t, "", gotOrgID, "Unexpected organization")
		assert.Equal(t, "", user, "Unexpected user")
		assert.Len(t, gotFeatureFlags, 0, "Unexpected organization")
	}
}

func TestMiddleware(t *testing.T) {
	server := dummyServer()
	defer server.Close()
	auth := client.MakeAuthenticator("web", server.URL, client.AuthenticatorOptions{})

	var (
		body                   = []byte("OK")
		headerName             = "X-Some-Header"
		featureFlagsHeaderName = "X-FeatureFlags"
	)

	testMiddleware := func(mw middleware.Interface, req *http.Request) *httptest.ResponseRecorder {
		wrapper := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, r.Header.Get(headerName), orgID, "")
			assert.Equal(t, r.Header.Get(featureFlagsHeaderName), strings.Join(featureFlags, " "), "")
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
			Authenticator:      auth,
			OutputHeader:       headerName,
			FeatureFlagsHeader: featureFlagsHeaderName,
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
			Authenticator:      auth,
			OrgExternalID:      func(*http.Request) (string, bool) { return orgExternalID, true },
			OutputHeader:       headerName,
			FeatureFlagsHeader: featureFlagsHeaderName,
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
			Authenticator:      auth,
			OrgExternalID:      func(*http.Request) (string, bool) { return orgExternalID, true },
			OutputHeader:       headerName,
			FeatureFlagsHeader: featureFlagsHeaderName,
		}
		result := testMiddleware(mw, req)
		assert.Equal(t, result.Code, http.StatusUnauthorized, "")
		assert.Equal(t, result.Body.Bytes(), []byte(nil), "")
	}

	// Test denying access for feature flags
	{
		req, _ := http.NewRequest("GET", "http://example.com/request?arg1=foo&arg2=bar", nil)
		req.Header.Set(client.AuthHeaderName, orgToken)
		mw := client.AuthProbeMiddleware{
			Authenticator:       auth,
			OutputHeader:        headerName,
			FeatureFlagsHeader:  featureFlagsHeaderName,
			RequireFeatureFlags: []string{"foo"},
		}
		result := testMiddleware(mw, req)
		assert.Equal(t, result.Code, http.StatusUnauthorized, "")
		assert.Equal(t, result.Body.Bytes(), []byte(nil), "")
	}
}
