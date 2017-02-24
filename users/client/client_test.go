package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"

	"github.com/weaveworks/common/middleware"
	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/tokens"
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

type dummyServer struct {
	*httptest.Server

	failRequests int32
	probeLookups int32
	orgLookups   int32
}

func newDummyServer() *dummyServer {
	d := &dummyServer{}
	serverHandler := mux.NewRouter()
	serverHandler.Methods("GET").Path("/private/api/users/lookup/{orgExternalID}").
		HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&d.orgLookups, 1)
			if atomic.LoadInt32(&d.failRequests) != 0 {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			localOrgExternalID := mux.Vars(r)["orgExternalID"]
			localOrgCookie, err := r.Cookie(AuthCookieName)
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
			atomic.AddInt32(&d.probeLookups, 1)
			if atomic.LoadInt32(&d.failRequests) != 0 {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			localAuthToken, ok := tokens.ExtractToken(r)
			if !ok {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
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
	d.Server = httptest.NewServer(serverHandler)
	return d
}

func TestAuth(t *testing.T) {
	server := newDummyServer()
	defer server.Close()
	auth, err := New("web", server.URL, CachingClientConfig{})
	assert.NoError(t, err)

	// Test we can auth based on the headers
	{
		response, err := auth.LookupUsingToken(context.Background(), &users.LookupUsingTokenRequest{
			Token: orgToken,
		})
		assert.NoError(t, err, "Unexpected error from authenticator")
		assert.Equal(t, orgID, response.OrganizationID, "Unexpected organization")
		assert.Equal(t, featureFlags, response.FeatureFlags, "Unexpected featureFlags")
	}

	// Test we can auth based on cookies
	{
		response, err := auth.LookupOrg(context.Background(), &users.LookupOrgRequest{
			Cookie:        orgCookie,
			OrgExternalID: orgExternalID,
		})
		assert.NoError(t, err, "Unexpected error from authenticator")
		assert.Equal(t, orgID, response.OrganizationID, "Unexpected organization")
		assert.Equal(t, userID, response.UserID, "Unexpected user")
		assert.Equal(t, featureFlags, response.FeatureFlags, "Unexpected organization")
	}

	// Test denying access for headers
	{
		response, err := auth.LookupUsingToken(context.Background(), &users.LookupUsingTokenRequest{
			Token: "This is not the right value",
		})
		assert.Error(t, err, "Unexpected successful authentication")
		assert.Equal(t, "", response.OrganizationID, "Unexpected organization")
		assert.Len(t, response.FeatureFlags, 0, "Unexpected organization")
	}

	// Test denying access for cookies
	{
		response, err := auth.LookupOrg(context.Background(), &users.LookupOrgRequest{
			Cookie:        "Not the right cookie",
			OrgExternalID: orgExternalID,
		})
		assert.Error(t, err, "Unexpected successful authentication")
		assert.Equal(t, "", response.OrganizationID, "Unexpected organization")
		assert.Equal(t, "", response.UserID, "Unexpected user")
		assert.Len(t, response.FeatureFlags, 0, "Unexpected organization")
	}
}

func TestAuthCache(t *testing.T) {
	server := newDummyServer()
	defer server.Close()
	auth, err := New("web", server.URL, CachingClientConfig{
		CredCacheEnabled:         true,
		ProbeCredCacheSize:       1,
		OrgCredCacheSize:         1,
		ProbeCredCacheExpiration: time.Minute,
		OrgCredCacheExpiration:   time.Minute,
	})
	assert.NoError(t, err)

	// Test denying access should be cached
	{
		response, err := auth.LookupUsingToken(context.Background(), &users.LookupUsingTokenRequest{
			Token: "This is not the right value",
		})
		assert.Error(t, err, "Unexpected successful authentication")
		assert.Equal(t, "", response.OrganizationID, "Unexpected organization")
		assert.Len(t, response.FeatureFlags, 0, "Unexpected organization")
		assert.Equal(t, int32(1), atomic.LoadInt32(&server.probeLookups), "Unexpected number of probe lookups")

		response, err = auth.LookupUsingToken(context.Background(), &users.LookupUsingTokenRequest{
			Token: "This is not the right value",
		})
		assert.Error(t, err, "Unexpected successful authentication")
		assert.Equal(t, "", response.OrganizationID, "Unexpected organization")
		assert.Len(t, response.FeatureFlags, 0, "Unexpected organization")
		assert.Equal(t, int32(1), atomic.LoadInt32(&server.probeLookups), "Unexpected number of probe lookups")
	}

	// Test other errors are not cached
	{
		atomic.StoreInt32(&server.failRequests, 1)

		response, err := auth.LookupUsingToken(context.Background(), &users.LookupUsingTokenRequest{
			Token: "This is a different but not right value",
		})
		assert.Error(t, err, "Unexpected successful authentication")
		assert.Equal(t, "", response.OrganizationID, "Unexpected organization")
		assert.Len(t, response.FeatureFlags, 0, "Unexpected organization")
		assert.Equal(t, int32(2), atomic.LoadInt32(&server.probeLookups), "Unexpected number of probe lookups")

		response, err = auth.LookupUsingToken(context.Background(), &users.LookupUsingTokenRequest{
			Token: "This is a different but not right value",
		})
		assert.Error(t, err, "Unexpected successful authentication")
		assert.Equal(t, "", response.OrganizationID, "Unexpected organization")
		assert.Len(t, response.FeatureFlags, 0, "Unexpected organization")
		assert.Equal(t, int32(3), atomic.LoadInt32(&server.probeLookups), "Unexpected number of probe lookups")
	}
}

func TestMiddleware(t *testing.T) {
	server := newDummyServer()
	defer server.Close()
	auth, err := New("web", server.URL, CachingClientConfig{})
	assert.NoError(t, err)

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
		req.Header.Set(tokens.AuthHeaderName, tokens.Prefix+orgToken)
		mw := AuthProbeMiddleware{
			UsersClient:        auth,
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
			Name:  AuthCookieName,
			Value: orgCookie,
		})
		mw := AuthOrgMiddleware{
			UsersClient:        auth,
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
		req.Header.Set(tokens.AuthHeaderName, "This is not the right value")
		mw := AuthProbeMiddleware{
			UsersClient:  auth,
			OutputHeader: headerName,
		}
		result := testMiddleware(mw, req)
		assert.Equal(t, result.Code, http.StatusUnauthorized, "")
		assert.Equal(t, result.Body.Bytes(), []byte(nil), "")
	}

	// Test denying access for cookies
	{
		req, _ := http.NewRequest("GET", "http://example.com/request?arg1=foo&arg2=bar", nil)
		req.AddCookie(&http.Cookie{
			Name:  AuthCookieName,
			Value: "Not the right cookie",
		})
		mw := AuthOrgMiddleware{
			UsersClient:        auth,
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
		req.Header.Set(tokens.AuthHeaderName, orgToken)
		mw := AuthProbeMiddleware{
			UsersClient:         auth,
			OutputHeader:        headerName,
			FeatureFlagsHeader:  featureFlagsHeaderName,
			RequireFeatureFlags: []string{"foo"},
		}
		result := testMiddleware(mw, req)
		assert.Equal(t, result.Code, http.StatusUnauthorized, "")
		assert.Equal(t, result.Body.Bytes(), []byte(nil), "")
	}
}
