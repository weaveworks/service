package client

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"github.com/weaveworks/common/middleware"
	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/render"
	"github.com/weaveworks/service/users/tokens"
)

const (
	orgExternalID = "%21?ЖЗИЙ%2FК%$?"
	orgID         = "somePersistentInternalID"
	orgToken      = "Scope-Probe token=token123"
	orgCookie     = "cookie123"
	userID        = "user12346"
	userEmail     = "user@example.org"
)

var (
	featureFlags = []string{"a", "b"}
)

type dummyServer struct {
	URL string
	users.UsersServer
	grpcServer *grpc.Server

	failRequests int32
	probeLookups int32
	orgLookups   int32
	userLookups  int32
}

// LookupOrg authenticates a cookie for access to an org by extenal ID.
func (d *dummyServer) LookupOrg(ctx context.Context, req *users.LookupOrgRequest) (*users.LookupOrgResponse, error) {
	atomic.AddInt32(&d.orgLookups, 1)
	if atomic.LoadInt32(&d.failRequests) != 0 {
		return nil, errors.New("fake error")
	}

	if req.OrgExternalID != orgExternalID || orgCookie != req.Cookie {
		return nil, users.ErrInvalidAuthenticationData
	}

	return &users.LookupOrgResponse{
		OrganizationID: orgID,
		UserID:         userID,
		FeatureFlags:   featureFlags,
	}, nil
}

// LookupUsingToken authenticates a token for access to an org.
func (d *dummyServer) LookupUsingToken(ctx context.Context, req *users.LookupUsingTokenRequest) (*users.LookupUsingTokenResponse, error) {
	atomic.AddInt32(&d.probeLookups, 1)
	if atomic.LoadInt32(&d.failRequests) != 0 {
		return nil, errors.New("fake error")
	}

	if orgToken != req.Token {
		return nil, users.ErrInvalidAuthenticationData
	}

	return &users.LookupUsingTokenResponse{
		OrganizationID: orgID,
		FeatureFlags:   featureFlags,
	}, nil
}

func (d *dummyServer) GetUser(ctx context.Context, req *users.GetUserRequest) (*users.GetUserResponse, error) {
	atomic.AddInt32(&d.userLookups, 1)
	if atomic.LoadInt32(&d.failRequests) != 0 {
		return nil, errors.New("fake error")
	}

	return &users.GetUserResponse{
		User: users.User{
			ID:    userID,
			Email: userEmail,
		},
	}, nil
}

func (d *dummyServer) Close() {
	d.grpcServer.GracefulStop()
}

func newDummyServer() (*dummyServer, error) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}

	d := &dummyServer{
		grpcServer: grpc.NewServer(grpc.UnaryInterceptor(render.GRPCErrorInterceptor)),
		URL:        "direct://" + lis.Addr().String(),
	}

	users.RegisterUsersServer(d.grpcServer, d)
	go d.grpcServer.Serve(lis)

	return d, nil
}

func TestAuth(t *testing.T) {
	server, err := newDummyServer()
	require.NoError(t, err)
	defer server.Close()
	auth, err := New("grpc", server.URL, CachingClientConfig{})
	require.NoError(t, err)

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
		require.Error(t, err, "Unexpected successful authentication")
		require.Nil(t, response)
	}

	// Test denying access for cookies
	{
		response, err := auth.LookupOrg(context.Background(), &users.LookupOrgRequest{
			Cookie:        "Not the right cookie",
			OrgExternalID: orgExternalID,
		})
		require.Error(t, err, "Unexpected successful authentication")
		require.Nil(t, response)
	}
}

func TestAuthCache(t *testing.T) {
	server, err := newDummyServer()
	require.NoError(t, err)
	defer server.Close()
	auth, err := New("grpc", server.URL, CachingClientConfig{
		CacheEnabled:             true,
		ProbeCredCacheSize:       1,
		OrgCredCacheSize:         1,
		UserCacheSize:            1,
		ProbeCredCacheExpiration: time.Minute,
		OrgCredCacheExpiration:   time.Minute,
		UserCacheExpiration:      time.Minute,
	})
	require.NoError(t, err)

	// Test denying access should be cached
	{
		response, err := auth.LookupUsingToken(context.Background(), &users.LookupUsingTokenRequest{
			Token: "This is not the right value",
		})
		assert.Error(t, err, "Unexpected successful authentication")
		assert.Nil(t, response)
		assert.Equal(t, int32(1), atomic.LoadInt32(&server.probeLookups), "Unexpected number of probe lookups")

		response, err = auth.LookupUsingToken(context.Background(), &users.LookupUsingTokenRequest{
			Token: "This is not the right value",
		})
		assert.Error(t, err, "Unexpected successful authentication")
		assert.Nil(t, response)
		assert.Equal(t, int32(1), atomic.LoadInt32(&server.probeLookups), "Unexpected number of probe lookups")
	}

	// Test other errors are not cached
	{
		atomic.StoreInt32(&server.failRequests, 1)

		response, err := auth.LookupUsingToken(context.Background(), &users.LookupUsingTokenRequest{
			Token: "This is a different but not right value",
		})
		assert.Error(t, err, "Unexpected successful authentication")
		assert.Nil(t, response)
		assert.Equal(t, int32(2), atomic.LoadInt32(&server.probeLookups), "Unexpected number of probe lookups")

		response, err = auth.LookupUsingToken(context.Background(), &users.LookupUsingTokenRequest{
			Token: "This is a different but not right value",
		})
		assert.Error(t, err, "Unexpected successful authentication")
		assert.Nil(t, response)
		assert.Equal(t, int32(3), atomic.LoadInt32(&server.probeLookups), "Unexpected number of probe lookups")
	}

	// Test GetUser cache
	{
		atomic.StoreInt32(&server.failRequests, 0)

		response, err := auth.GetUser(context.Background(), &users.GetUserRequest{
			UserID: userID,
		})
		assert.NoError(t, err)
		assert.Equal(t, response.User.ID, "user12346")
		assert.Equal(t, response.User.Email, "user@example.org")
		assert.Equal(t, int32(1), atomic.LoadInt32(&server.userLookups), "Unexpected number of user lookups")

		response, err = auth.GetUser(context.Background(), &users.GetUserRequest{
			UserID: userID,
		})
		assert.NoError(t, err)
		assert.Equal(t, response.User.ID, "user12346")
		assert.Equal(t, response.User.Email, "user@example.org")
		assert.Equal(t, int32(1), atomic.LoadInt32(&server.userLookups), "Unexpected number of user lookups")
	}
}

func TestMiddleware(t *testing.T) {
	server, err := newDummyServer()
	require.NoError(t, err)
	defer server.Close()
	auth, err := New("grpc", server.URL, CachingClientConfig{})
	require.NoError(t, err)

	var (
		body                   = []byte("OK")
		headerName             = "X-Scope-OrgID"
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
			UsersClient: auth,
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
			FeatureFlagsHeader: featureFlagsHeaderName,
		}
		result := testMiddleware(mw, req)
		assert.Equal(t, result.Code, http.StatusUnauthorized)
		assert.Equal(t, result.Body.String(), "Unauthorized\n")
	}

	// Test denying access for feature flags
	{
		req, _ := http.NewRequest("GET", "http://example.com/request?arg1=foo&arg2=bar", nil)
		req.Header.Set(tokens.AuthHeaderName, orgToken)
		mw := AuthProbeMiddleware{
			UsersClient:         auth,
			FeatureFlagsHeader:  featureFlagsHeaderName,
			RequireFeatureFlags: []string{"foo"},
		}
		result := testMiddleware(mw, req)
		assert.Equal(t, result.Code, http.StatusUnauthorized)
		assert.Equal(t, result.Body.String(), "Unauthorized\n")
	}
}
