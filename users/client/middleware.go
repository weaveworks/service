package client

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/google/go-github/github"
	"github.com/gorilla/mux"
	"github.com/weaveworks/common/httpgrpc"
	"github.com/weaveworks/common/logging"
	"github.com/weaveworks/common/user"
	"github.com/weaveworks/service/common/constants/webhooks"
	"github.com/weaveworks/service/common/featureflag"
	httpUtil "github.com/weaveworks/service/common/http"
	"github.com/weaveworks/service/common/permission"
	"github.com/weaveworks/service/common/tracing"
	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/tokens"

	opentracing "github.com/opentracing/opentracing-go"
)

// Constants exported for testing
const (
	AuthCookieName = "_weave_scope_session"

	// Impersonation cookie present only when there is an impersonation - absence implies user is real
	// It only ever contains an empty string
	// If creation/deletion needed, will happen at same time as session cookie is operated on
	ImpersonationCookieName = "_weave_cloud_impersonation"

	userTag = "user"
	orgTag  = "organization"
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
		ctx := r.Context()
		logger := user.LogWith(ctx, logging.Global())
		orgExternalID, ok := a.OrgExternalID(r)
		if !ok {
			logger.Errorf("Invalid request, no org id: %s", r.RequestURI)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		authCookie, err := r.Cookie(AuthCookieName)
		if err != nil {
			logger.Errorf("Unauthorised request, no auth cookie: %v", err)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		response, err := a.UsersClient.LookupOrg(ctx, &users.LookupOrgRequest{
			Cookie:        authCookie.Value,
			OrgExternalID: orgExternalID,
			AuthorizeFor:  a.AuthorizeFor,
		})
		if err != nil {
			handleError(err, w, r)
			return
		}

		tracing.ForceTraceIfFlagged(ctx, r, response.FeatureFlags) // must do this before setting tags
		if span := opentracing.SpanFromContext(r.Context()); span != nil {
			span.SetTag(userTag, response.UserID)
			span.SetTag(orgTag, response.OrganizationID)
		}

		if !featureflag.HasFeatureAllFlags(a.RequireFeatureFlags, response.FeatureFlags) {
			logger.Errorf("Unauthorised request, missing feature flags: %v", a.RequireFeatureFlags)
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
		ctx := r.Context()
		logger := user.LogWith(ctx, logging.Global())
		token, ok := tokens.ExtractToken(r)
		if !ok {
			logger.WithField("host", httpUtil.HostFromRequest(r)).WithField("url", r.URL.Path).Infof("Unauthorised probe request, no token")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		ok = utf8.Valid([]byte(token))
		if !ok {
			logger.Errorf("Invalid token. Not valid utf8: %v", base64.StdEncoding.EncodeToString([]byte(token)))
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		response, err := a.UsersClient.LookupUsingToken(ctx, &users.LookupUsingTokenRequest{
			Token:        token,
			AuthorizeFor: a.AuthorizeFor,
		})
		if err != nil {
			handleError(err, w, r)
			return
		}

		tracing.ForceTraceIfFlagged(ctx, r, response.FeatureFlags) // must do this before setting tags
		if span := opentracing.SpanFromContext(r.Context()); span != nil {
			span.SetTag(orgTag, response.OrganizationID)
		}

		if !featureflag.HasFeatureAllFlags(a.RequireFeatureFlags, response.FeatureFlags) {
			logger.Errorf("Unauthorised probe request, missing feature flags: %v", a.RequireFeatureFlags)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		r.Header.Add(a.FeatureFlagsHeader, strings.Join(response.FeatureFlags, " "))

		finishRequest(next, w, r, response.OrganizationID)
	})
}

// AuthAdminMiddleware is a middleware.Interface for authentication probes based on the headers
type AuthAdminMiddleware struct {
	UsersClient users.UsersClient
}

// Wrap implements middleware.Interface
func (a AuthAdminMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := user.LogWith(ctx, logging.Global())
		authCookie, err := r.Cookie(AuthCookieName)
		if err != nil {
			logger.WithField("host", httpUtil.HostFromRequest(r)).Errorf("Unauthorised admin request, no auth cookie: %v", err)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		response, err := a.UsersClient.LookupAdmin(ctx, &users.LookupAdminRequest{
			Cookie: authCookie.Value,
		})
		if err != nil {
			handleError(err, w, r)
			return
		}
		if span := opentracing.SpanFromContext(ctx); span != nil {
			span.SetTag(userTag, response.AdminID)
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
		ctx := r.Context()
		authCookie, err := r.Cookie(AuthCookieName)
		if err != nil {
			user.LogWith(ctx, logging.Global()).WithField("host", httpUtil.HostFromRequest(r)).WithField("url", r.URL.Path).Infof("Unauthorised user request, no auth cookie: %v", err)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		response, err := a.UsersClient.LookupUser(ctx, &users.LookupUserRequest{
			Cookie: authCookie.Value,
		})

		if err != nil {
			handleError(err, w, r)
			return
		}
		if span := opentracing.SpanFromContext(r.Context()); span != nil {
			span.SetTag(userTag, response.UserID)
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
	logger := user.LogWith(r.Context(), logging.Global())
	if errResp, ok := httpgrpc.HTTPResponseFromError(err); ok {
		switch errResp.Code {
		case http.StatusUnauthorized:
			// If clients can tell the difference between invalid login, and login not
			// found, our API has a user membership check vulnerability
			// To prevent this, don't send on the actual message.
			http.Error(w, "Unauthorized", int(errResp.Code))
		case http.StatusForbidden:
			fallthrough
		case http.StatusPaymentRequired:
			http.Error(w, string(errResp.Body), int(errResp.Code))
		case http.StatusNotFound:
			http.Error(w, string(errResp.Body), int(errResp.Code))
		default:
			logger.Errorf("Error from users svc: %v (%d)", string(errResp.Body), errResp.Code)
			w.WriteHeader(http.StatusUnauthorized)
		}
	} else {
		logger.Errorf("Error talking to users svc: %v", err)
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
			user.LogWith(r.Context(), logging.Global()).WithField("host", httpUtil.HostFromRequest(r)).Infof("Unauthorised secret request, secret mismatch: %v", secret)
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func validateGCPExternalAccountID(externalAccountID string) (string, error) {
	if match, _ := regexp.MatchString(`^[0-9A-Fa-f]{1}\-[0-9A-Fa-f]{4}\-[0-9A-Fa-f]{4}\-[0-9A-Fa-f]{4}\-[0-9A-Fa-f]{4}$`, externalAccountID); !match {
		return "", fmt.Errorf("invalid GCP account ID [%v]: malformed", externalAccountID)
	}
	return externalAccountID, nil
}

// WebhooksMiddleware is a middleware.Interface for authentication request based
// on the webhook secret (and signing key if one exists).
type WebhooksMiddleware struct {
	UsersClient                   users.UsersClient
	WebhooksIntegrationTypeHeader string
}

// Wrap implements middleware.Interface
func (a WebhooksMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secretID := mux.Vars(r)["secretID"]

		// Verify the secretID
		response, err := a.UsersClient.LookupOrganizationWebhookUsingSecretID(r.Context(), &users.LookupOrganizationWebhookUsingSecretIDRequest{
			SecretID: secretID,
		})
		if err != nil {
			handleError(err, w, r)
			return
		}

		// Verify the signature if we require it. Only Github and
		// Gitlab integrations use this, in different ways (Bitbucket
		// Cloud does not support it).
		switch response.Webhook.IntegrationType {
		case webhooks.GithubPushIntegrationType:
			if response.Webhook.SecretSigningKey == "" {
				http.Error(w, "The GitHub signing key is missing.", 500)
				return
			}
			// Validating the payload consumes the request Body; so we
			// will need to replace it afterwards.
			body, err := ioutil.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "Could not read request body: "+err.Error(), http.StatusBadRequest)
				return
			}
			r.Body = ioutil.NopCloser(bytes.NewReader(body))
			_, err = github.ValidatePayload(r, []byte(response.Webhook.SecretSigningKey))
			if err != nil {
				http.Error(w, "The GitHub signature header is invalid.", 401)
				return
			}
			r.Body = ioutil.NopCloser(bytes.NewReader(body))
		case webhooks.GitlabPushIntegrationType:
			if response.Webhook.SecretSigningKey == "" {
				http.Error(w, "The Gitlab shared secret is missing", 500)
				return
			}
			if r.Header.Get("X-Gitlab-Token") != response.Webhook.SecretSigningKey {
				http.Error(w, "The Gitlab token does not match", 401)
				return
			}
		}

		// Set the FirstSeenAt time if it is not set
		if response.Webhook.FirstSeenAt == nil {
			_, err := a.UsersClient.SetOrganizationWebhookFirstSeenAt(r.Context(), &users.SetOrganizationWebhookFirstSeenAtRequest{
				SecretID: secretID,
			})
			if err != nil {
				handleError(err, w, r)
				return
			}
		}

		// Add the integration type and org ID to the headers for use by flux-api.
		r.Header.Add(a.WebhooksIntegrationTypeHeader, response.Webhook.IntegrationType)
		finishRequest(next, w, r, response.Webhook.OrganizationID)
	})
}

// UserPermissionsMiddleware is a middleware.Interface which grants permissions based on team member role.
type UserPermissionsMiddleware struct {
	UsersClient  users.UsersClient
	UserIDHeader string
}

// Wrap implements middleware.Interface
func (a UserPermissionsMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, p := range []struct {
			Path         string
			Methods      []string
			PermissionID string
		}{
			// Prometheus
			{"/api/prom/configs/rules", []string{"POST"}, permission.UpdateAlertingSettings},
			// Scope
			{"/api/control/.*/.*/host_exec", []string{"POST"}, permission.OpenHostShell},
			{"/api/control/.*/.*/docker_exec_container", []string{"POST"}, permission.OpenContainerShell},
			{"/api/control/.*/.*/docker_attach_container", []string{"POST"}, permission.AttachToContainer},
			{"/api/control/.*/.*/docker_(pause|unpause)_container", []string{"POST"}, permission.PauseContainer},
			{"/api/control/.*/.*/docker_restart_container", []string{"POST"}, permission.RestartContainer},
			{"/api/control/.*/.*/docker_stop_container", []string{"POST"}, permission.StopContainer},
			{"/api/control/.*/.*/kubernetes_get_logs", []string{"POST"}, permission.ViewPodLogs},
			{"/api/control/.*/.*/kubernetes_scale_(up|down)", []string{"POST"}, permission.UpdateReplicaCount},
			{"/api/control/.*/.*/kubernetes_delete_pod", []string{"POST"}, permission.DeletePod},
			// Flux
			// TODO(fbarl): At the moment, `update-manifests` API is only used for pushing releases in the Flux UI,
			// so setting the permission here works, but in the future, we should probably introduce case branching.
			{"/api/flux/v9/update-manifests", []string{"POST"}, permission.DeployImage},
			{"/api/flux/v6/update-images", []string{"POST"}, permission.DeployImage},
			{"/api/flux/v6/policies", []string{"PATCH"}, permission.UpdateDeploymentPolicy},
			// Notifications
			{"/api/notification/config/.*", []string{"POST", "PUT"}, permission.UpdateNotificationSettings},
		} {
			// TODO(fbarl): Find a better way to check for the API route.
			URIMatched := regexp.MustCompile(p.Path).MatchString(r.URL.String())

			MethodMatched := false
			for _, m := range p.Methods {
				if m == r.Method {
					MethodMatched = true
				}
			}

			if MethodMatched && URIMatched {
				if _, err := a.UsersClient.RequireOrgMemberPermissionTo(r.Context(), &users.RequireOrgMemberPermissionToRequest{
					OrgID:        &users.RequireOrgMemberPermissionToRequest_OrgExternalID{OrgExternalID: mux.Vars(r)["orgExternalID"]},
					UserID:       r.Header.Get(a.UserIDHeader),
					PermissionID: p.PermissionID,
				}); err != nil {
					handleError(err, w, r)
					return
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

// ScopeCensorMiddleware is a middleware.Interface which passes
// certain query params to Scope API depending on user permissions.
type ScopeCensorMiddleware struct {
	UsersClient  users.UsersClient
	UserIDHeader string
}

// Wrap implements middleware.Interface
func (a ScopeCensorMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// If the user has no permission to view the token, we tell Scope to hide all the sensitive data for the request.
		if _, err := a.UsersClient.RequireOrgMemberPermissionTo(r.Context(), &users.RequireOrgMemberPermissionToRequest{
			OrgID:        &users.RequireOrgMemberPermissionToRequest_OrgExternalID{OrgExternalID: mux.Vars(r)["orgExternalID"]},
			UserID:       r.Header.Get(a.UserIDHeader),
			PermissionID: permission.ViewToken,
		}); err != nil {
			q := r.URL.Query()
			q.Set("hideCommandLineArguments", "true")
			q.Set("hideEnvironmentVariables", "true")
			r.URL.RawQuery = q.Encode()
		}
		next.ServeHTTP(w, r)
	})
}
