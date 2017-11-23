package client

import (
	"crypto/sha1"
	"fmt"
	"io"
	"net/http"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/weaveworks/common/httpgrpc"
	"github.com/weaveworks/common/logging"
	"github.com/weaveworks/common/user"
	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/tokens"
)

// Constants exported for testing
const (
	AuthCookieName = "_weave_scope_session"

	// Impersonation cookie present only when there is an impersonation - absence implies user is real
	// It only ever contains an empty string
	// If creation/deletion needed, will happen at same time as session cookie is operated on
	ImpersonationCookieName = "_weave_cloud_impersonation"
)

// AuthOrgMiddleware is a middleware.Interface for authentication organisations based on the
// cookie and an org name in the path
type AuthOrgMiddleware struct {
	UsersClient   users.UsersClient
	OrgExternalID func(*http.Request) (string, bool)

	UserIDHeader        string
	FeatureFlagsHeader  string
	RequireFeatureFlags []string
	AuthorizeFor        users.AuthorizedAction
}

// Wrap implements middleware.Interface
func (a AuthOrgMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		orgExternalID, ok := a.OrgExternalID(r)
		if !ok {
			logging.With(r.Context()).Errorf("Invalid request, no org id: %s", r.RequestURI)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		authCookie, err := r.Cookie(AuthCookieName)
		if err != nil {
			logging.With(r.Context()).Errorf("Unauthorised request, no auth cookie: %v", err)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		response, err := a.UsersClient.LookupOrg(r.Context(), &users.LookupOrgRequest{
			Cookie:        authCookie.Value,
			OrgExternalID: orgExternalID,
			AuthorizeFor:  a.AuthorizeFor,
		})
		if err != nil {
			handleError(err, w, r)
			return
		}

		if !hasFeatureAllFlags(a.RequireFeatureFlags, response.FeatureFlags) {
			logging.With(r.Context()).Errorf("Unauthorised request, missing feature flags: %v", a.RequireFeatureFlags)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		r.Header.Add(a.UserIDHeader, response.UserID)
		r.Header.Add(a.FeatureFlagsHeader, strings.Join(response.FeatureFlags, " "))

		finishRequest(next, w, r, response.OrganizationID)
	})
}

// AuthProbeMiddleware is a middleware.Interface for authentication probes based on the headers
type AuthProbeMiddleware struct {
	UsersClient         users.UsersClient
	FeatureFlagsHeader  string
	RequireFeatureFlags []string
	AuthorizeFor        users.AuthorizedAction
}

// Wrap implements middleware.Interface
func (a AuthProbeMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := tokens.ExtractToken(r)
		if !ok {
			logging.With(r.Context()).Errorf("Unauthorised probe request, no token")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		response, err := a.UsersClient.LookupUsingToken(r.Context(), &users.LookupUsingTokenRequest{
			Token:        token,
			AuthorizeFor: a.AuthorizeFor,
		})
		if err != nil {
			handleError(err, w, r)
			return
		}

		if !hasFeatureAllFlags(a.RequireFeatureFlags, response.FeatureFlags) {
			logging.With(r.Context()).Errorf("Unauthorised probe request, missing feature flags: %v", a.RequireFeatureFlags)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		r.Header.Add(a.FeatureFlagsHeader, strings.Join(response.FeatureFlags, " "))

		finishRequest(next, w, r, response.OrganizationID)
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
			logging.With(r.Context()).Errorf("Unauthorised admin request, no auth cookie: %v", err)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		response, err := a.UsersClient.LookupAdmin(r.Context(), &users.LookupAdminRequest{
			Cookie: authCookie.Value,
		})
		if err != nil {
			handleError(err, w, r)
			return
		}

		finishRequest(next, w, r, response.AdminID)
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
			logging.With(r.Context()).Infof("Unauthorised user request, no auth cookie: %v", err)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		response, err := a.UsersClient.LookupUser(r.Context(), &users.LookupUserRequest{
			Cookie: authCookie.Value,
		})
		if err != nil {
			handleError(err, w, r)
			return
		}

		r.Header.Add(a.UserIDHeader, response.UserID)
		next.ServeHTTP(w, r)
	})
}

func finishRequest(next http.Handler, w http.ResponseWriter, r *http.Request, orgID string) {
	r = r.WithContext(user.InjectOrgID(r.Context(), orgID))
	if err := user.InjectOrgIDIntoHTTPRequest(r.Context(), r); err != nil {
		handleError(err, w, r)
	} else {
		next.ServeHTTP(w, r)
	}
}

func handleError(err error, w http.ResponseWriter, r *http.Request) {
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
			logging.With(r.Context()).Errorf("Error from users svc: %v (%d)", string(errResp.Body), errResp.Code)
			w.WriteHeader(http.StatusUnauthorized)
		}
	} else {
		logging.With(r.Context()).Errorf("Error talking to users svc: %v", err)
		w.WriteHeader(http.StatusBadGateway)
	}
}

// AuthSecretMiddleware is a middleware for authentication based on a shared secret.
type AuthSecretMiddleware struct {
	Secret string
}

// Wrap implements middleware.Interface
func (a AuthSecretMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secret := r.URL.Query().Get("secret")
		// Deny access if no secret is configured or secret does not match
		if a.Secret == "" || secret != a.Secret {
			logging.With(r.Context()).Infof("Unauthorised secret request, secret mismatch: %v", secret)
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// GCPLoginSecretMiddleware validates incoming GCP SSO requests based on a shared secret.
type GCPLoginSecretMiddleware struct {
	Secret string
}

// Arbitrary minimum value to validate the provided timestamp.
// In this instance: Mon Nov 13 2017 14:11:32, i.e. way before this service was put in production.
const minTimestampInMillis = 1510582292911

// Tokenise returns the checksum of SHA1("keyForSsoLogin:secret:timestampInMillis").
// This is useful to verify the authenticity of incoming requests.
func (m GCPLoginSecretMiddleware) Tokenise(keyForSsoLogin, timestampInMillis string) string {
	h := sha1.New()
	io.WriteString(h, keyForSsoLogin)
	io.WriteString(h, ":")
	io.WriteString(h, m.Secret)
	io.WriteString(h, ":")
	io.WriteString(h, timestampInMillis)
	return fmt.Sprintf("%x", h.Sum(nil))
}

// Wrap implements middleware.Interface
func (m GCPLoginSecretMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if status, err := m.validate(r); err != nil {
			logging.With(r.Context()).Warnf("Unauthorised request: %v", err)
			http.Error(w, http.StatusText(status), status)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (m GCPLoginSecretMiddleware) validate(r *http.Request) (int, error) {
	ts, err := validateTimestamp(r.URL.Query().Get("timestamp"))
	if err != nil {
		return http.StatusBadRequest, err
	}
	token, err := validateToken(r.URL.Query().Get("ssoToken"))
	if err != nil {
		return http.StatusBadRequest, err
	}
	externalAccountID, err := validateGCPExternalAccountID(path.Base(r.URL.Path))
	if err != nil {
		return http.StatusBadRequest, err
	}
	expectedToken := m.Tokenise(externalAccountID, ts)
	if token != expectedToken {
		return http.StatusUnauthorized, fmt.Errorf("invalid token [%v], expected [%v] for request [%v]", token, expectedToken, r)
	}
	return http.StatusOK, nil
}

func validateTimestamp(timestampInMillis string) (string, error) {
	if len(timestampInMillis) <= 0 {
		return "", errors.New("empty timestamp")
	}
	ts, err := strconv.ParseUint(timestampInMillis, 10, 64)
	if err != nil {
		return "", errors.Wrapf(err, "invalid timestamp [%v]: ", timestampInMillis)
	}
	if ts <= minTimestampInMillis {
		return "", fmt.Errorf("invalid timestamp [%v]", timestampInMillis)
	}
	return timestampInMillis, nil
}

func validateToken(token string) (string, error) {
	if match, _ := regexp.MatchString(`^[0-9A-Fa-f]{40}$`, token); !match {
		return "", fmt.Errorf("invalid token [%v]: malformed", token)
	}
	return token, nil
}

func validateGCPExternalAccountID(externalAccountID string) (string, error) {
	if match, _ := regexp.MatchString(`^[0-9A-Fa-f]{1}\-[0-9A-Fa-f]{4}\-[0-9A-Fa-f]{4}\-[0-9A-Fa-f]{4}\-[0-9A-Fa-f]{4}$`, externalAccountID); !match {
		return "", fmt.Errorf("invalid GCP account ID [%v]: malformed", externalAccountID)
	}
	return externalAccountID, nil
}
