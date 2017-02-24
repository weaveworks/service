package client

import (
	"net/http"
	"strings"

	log "github.com/Sirupsen/logrus"

	"github.com/weaveworks/common/user"
	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/tokens"
)

// Constants exported for testing
const (
	AuthCookieName = "_weave_scope_session"
)

// AuthOrgMiddleware is a middleware.Interface for authentication organisations based on the
// cookie and an org name in the path
type AuthOrgMiddleware struct {
	UsersClient         users.UsersClient
	OrgExternalID       func(*http.Request) (string, bool)
	OutputHeader        string
	UserIDHeader        string
	FeatureFlagsHeader  string
	RequireFeatureFlags []string
}

// Wrap implements middleware.Interface
func (a AuthOrgMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		orgExternalID, ok := a.OrgExternalID(r)
		if !ok {
			log.Infof("invalid request - no org id: %s", r.RequestURI)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		authCookie, err := r.Cookie(AuthCookieName)
		if err != nil {
			log.Infof("unauthorised request - no auth cookie: %v", err)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		response, err := a.UsersClient.LookupOrg(r.Context(), &users.LookupOrgRequest{
			Cookie:        authCookie.Value,
			OrgExternalID: orgExternalID,
		})
		if err != nil {
			if unauth, ok := err.(*Unauthorized); ok {
				log.Infof("unauthorized request: %d", unauth.httpStatus)
				w.WriteHeader(http.StatusUnauthorized)
			} else {
				log.Errorf("error contacting authenticator: %v", err)
				w.WriteHeader(http.StatusBadGateway)
			}
			return
		}

		if !hasFeatureAllFlags(a.RequireFeatureFlags, response.FeatureFlags) {
			log.Infof("proxy: missing feature flags: %v", a.RequireFeatureFlags)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		r.Header.Add(a.OutputHeader, response.OrganizationID)
		r.Header.Add(a.UserIDHeader, response.UserID)
		r.Header.Add(a.FeatureFlagsHeader, strings.Join(response.FeatureFlags, " "))
		next.ServeHTTP(w, r.WithContext(user.WithID(r.Context(), response.OrganizationID)))
	})
}

// AuthProbeMiddleware is a middleware.Interface for authentication probes based on the headers
type AuthProbeMiddleware struct {
	UsersClient         users.UsersClient
	OutputHeader        string
	FeatureFlagsHeader  string
	RequireFeatureFlags []string
}

// Wrap implements middleware.Interface
func (a AuthProbeMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := tokens.ExtractToken(r)
		if !ok {
			log.Infof("unauthorised request - no token")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		response, err := a.UsersClient.LookupUsingToken(r.Context(), &users.LookupUsingTokenRequest{
			Token: token,
		})
		if err != nil {
			if unauth, ok := err.(*Unauthorized); ok {
				log.Infof("proxy: unauthorized request: %d", unauth.httpStatus)
				w.WriteHeader(http.StatusUnauthorized)
			} else {
				log.Errorf("proxy: error contacting authenticator: %v", err)
				w.WriteHeader(http.StatusBadGateway)
			}
			return
		}

		if !hasFeatureAllFlags(a.RequireFeatureFlags, response.FeatureFlags) {
			log.Infof("proxy: missing feature flags: %v", a.RequireFeatureFlags)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		r.Header.Add(a.OutputHeader, response.OrganizationID)
		r.Header.Add(a.FeatureFlagsHeader, strings.Join(response.FeatureFlags, " "))
		next.ServeHTTP(w, r.WithContext(user.WithID(r.Context(), response.OrganizationID)))
	})
}

func hasFeatureAllFlags(needles, haystack []string) bool {
	for _, f := range needles {
		found := false
		for _, has := range haystack {
			if f == has {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// AuthAdminMiddleware is a middleware.Interface for authentication probes based on the headers
type AuthAdminMiddleware struct {
	UsersClient  users.UsersClient
	OutputHeader string
}

// Wrap implements middleware.Interface
func (a AuthAdminMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authCookie, err := r.Cookie(AuthCookieName)
		if err != nil {
			log.Infof("unauthorised request - no auth cookie: %v", err)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		response, err := a.UsersClient.LookupAdmin(r.Context(), &users.LookupAdminRequest{
			Cookie: authCookie.Value,
		})
		if err != nil {
			if unauth, ok := err.(*Unauthorized); ok {
				log.Infof("proxy: unauthorized request: %d", unauth.httpStatus)
				w.WriteHeader(http.StatusUnauthorized)
			} else {
				log.Errorf("proxy: error contacting authenticator: %v", err)
				w.WriteHeader(http.StatusBadGateway)
			}
			return
		}

		r.Header.Add(a.OutputHeader, response.AdminID)
		next.ServeHTTP(w, r.WithContext(user.WithID(r.Context(), response.AdminID)))
	})
}

// AuthUserMiddleware is a middleware.Interface for authentication users based on the
// cookie (and not to any specific org)
type AuthUserMiddleware struct {
	UsersClient         users.UsersClient
	UserIDHeader        string
	FeatureFlagsHeader  string
	RequireFeatureFlags []string
}

// Wrap implements middleware.Interface
func (a AuthUserMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authCookie, err := r.Cookie(AuthCookieName)
		if err != nil {
			log.Infof("unauthorised request - no auth cookie: %v", err)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		response, err := a.UsersClient.LookupUser(r.Context(), &users.LookupUserRequest{
			Cookie: authCookie.Value,
		})
		if err != nil {
			if unauth, ok := err.(*Unauthorized); ok {
				log.Infof("proxy: unauthorized request: %d", unauth.httpStatus)
				w.WriteHeader(http.StatusUnauthorized)
			} else {
				log.Errorf("proxy: error contacting authenticator: %v", err)
				w.WriteHeader(http.StatusBadGateway)
			}
			return
		}

		r.Header.Add(a.UserIDHeader, response.UserID)
		next.ServeHTTP(w, r.WithContext(user.WithID(r.Context(), response.UserID)))
	})
}
