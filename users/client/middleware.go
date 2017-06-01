package client

import (
	"net/http"
	"strings"

	log "github.com/Sirupsen/logrus"

	"github.com/weaveworks/common/httpgrpc"
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
	UsersClient   users.UsersClient
	OrgExternalID func(*http.Request) (string, bool)

	UserIDHeader           string
	FeatureFlagsHeader     string
	RequireFeatureFlags    []string
	AuthorizeForUIFeatures bool
}

// Wrap implements middleware.Interface
func (a AuthOrgMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		orgExternalID, ok := a.OrgExternalID(r)
		if !ok {
			log.Errorf("Invalid request, no org id: %s", r.RequestURI)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		authCookie, err := r.Cookie(AuthCookieName)
		if err != nil {
			log.Errorf("Unauthorised request, no auth cookie: %v", err)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		response, err := a.UsersClient.LookupOrg(r.Context(), &users.LookupOrgRequest{
			Cookie:                 authCookie.Value,
			OrgExternalID:          orgExternalID,
			AuthorizeForUIFeatures: a.AuthorizeForUIFeatures,
		})
		if err != nil {
			handleError(err, w)
			return
		}

		if !hasFeatureAllFlags(a.RequireFeatureFlags, response.FeatureFlags) {
			log.Errorf("Unauthorised request, missing feature flags: %v", a.RequireFeatureFlags)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		r.Header.Add(a.UserIDHeader, response.UserID)
		r.Header.Add(a.FeatureFlagsHeader, strings.Join(response.FeatureFlags, " "))

		r = r.WithContext(user.InjectOrgID(r.Context(), response.OrganizationID))
		user.InjectOrgIDIntoHTTPRequest(r.Context(), r)
		next.ServeHTTP(w, r)
	})
}

// AuthProbeMiddleware is a middleware.Interface for authentication probes based on the headers
type AuthProbeMiddleware struct {
	UsersClient         users.UsersClient
	FeatureFlagsHeader  string
	RequireFeatureFlags []string
}

// Wrap implements middleware.Interface
func (a AuthProbeMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := tokens.ExtractToken(r)
		if !ok {
			log.Errorf("Unauthorised request, no token")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		response, err := a.UsersClient.LookupUsingToken(r.Context(), &users.LookupUsingTokenRequest{
			Token: token,
		})
		if err != nil {
			handleError(err, w)
			return
		}

		if !hasFeatureAllFlags(a.RequireFeatureFlags, response.FeatureFlags) {
			log.Errorf("Unauthorised request, missing feature flags: %v", a.RequireFeatureFlags)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		r.Header.Add(a.FeatureFlagsHeader, strings.Join(response.FeatureFlags, " "))

		r = r.WithContext(user.InjectOrgID(r.Context(), response.OrganizationID))
		user.InjectOrgIDIntoHTTPRequest(r.Context(), r)
		next.ServeHTTP(w, r)
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
	UsersClient users.UsersClient
}

// Wrap implements middleware.Interface
func (a AuthAdminMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authCookie, err := r.Cookie(AuthCookieName)
		if err != nil {
			log.Errorf("Unauthorised request, no auth cookie: %v", err)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		response, err := a.UsersClient.LookupAdmin(r.Context(), &users.LookupAdminRequest{
			Cookie: authCookie.Value,
		})
		if err != nil {
			handleError(err, w)
			return
		}

		r = r.WithContext(user.InjectOrgID(r.Context(), response.AdminID))
		user.InjectOrgIDIntoHTTPRequest(r.Context(), r)
		next.ServeHTTP(w, r)
	})
}

// AuthUserMiddleware is a middleware.Interface for authentication users based on the
// cookie (and not to any specific org)
type AuthUserMiddleware struct {
	UsersClient         users.UsersClient
	UserIDHeader        string
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
			handleError(err, w)
			return
		}

		r.Header.Add(a.UserIDHeader, response.UserID)
		next.ServeHTTP(w, r)
	})
}

func handleError(err error, w http.ResponseWriter) {
	if errResp, ok := httpgrpc.HTTPResponseFromError(err); ok {
		switch errResp.Code {
		case http.StatusUnauthorized:
			// If clients can tell the difference between invalid login, and login not
			// found, our API has a user membership check vulnerability
			// To prevent this, don't send on the actual message.
			http.Error(w, "Unauthorized", int(errResp.Code))
		case http.StatusPaymentRequired:
			http.Error(w, string(errResp.Body), int(errResp.Code))
		default:
			log.Errorf("Error from users svc: %v (%d)", string(errResp.Body), errResp.Code)
			w.WriteHeader(http.StatusUnauthorized)
		}
	} else {
		log.Errorf("Error talking to users svc: %v", err)
		w.WriteHeader(http.StatusBadGateway)
	}
}
