package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"mime"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/weaveworks/service/common/gcp"

	"github.com/gorilla/mux"
	"github.com/justinas/nosurf"
	"github.com/opentracing-contrib/go-stdlib/nethttp"
	opentracing "github.com/opentracing/opentracing-go"

	"github.com/weaveworks/common/logging"
	"github.com/weaveworks/common/middleware"
	"github.com/weaveworks/common/user"
	"github.com/weaveworks/service/common"
	"github.com/weaveworks/service/common/constants/webhooks"
	"github.com/weaveworks/service/common/featureflag"
	"github.com/weaveworks/service/common/permission"
	"github.com/weaveworks/service/users"
	users_client "github.com/weaveworks/service/users/client"
)

const maxAnalyticsPayloadSize = 16 * 1024 // bytes

func (c Config) commonMiddleWare(routeMatcher middleware.RouteMatcher) middleware.Interface {
	extraHeaders := http.Header{}

	// Common security headers
	extraHeaders.Add("X-Frame-Options", "SAMEORIGIN")
	extraHeaders.Add("X-XSS-Protection", "1; mode=block")
	extraHeaders.Add("X-Content-Type-Options", "nosniff")
	extraHeaders.Add("Referrer-Policy", "origin-when-cross-origin")

	if c.sendCSPHeader {
		extraHeaders.Add("Content-Security-Policy", "default-src https: 'unsafe-inline'")
	}
	if c.redirectHTTPS && c.hstsMaxAge > 0 {
		extraHeaders.Add("Strict-Transport-Security", fmt.Sprintf("max-age=%d; includeSubDomains", c.hstsMaxAge))
	}
	return middleware.Merge(
		middleware.HeaderAdder{
			Header: extraHeaders,
		},
		middleware.Instrument{
			RouteMatcher: routeMatcher,
			Duration:     common.RequestDuration,
		},
		middleware.Log{
			Log: logging.Global(),
		},
	)
}

var noopHandler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNoContent)
})

func routes(c Config, authenticator users.UsersClient, ghIntegration *users_client.TokenRequester, eventLogger *EventLogger) (http.Handler, error) {
	launcherServiceLogger, probeHTTPlogger, uiHTTPlogger, analyticsLogger, webhooksLogger := middleware.Identity, middleware.Identity, middleware.Identity, middleware.Identity, middleware.Identity
	if eventLogger != nil {
		launcherServiceLogger = HTTPEventLogger{
			Extractor: newLauncherServiceLogger(authenticator),
			Logger:    eventLogger,
		}
		probeHTTPlogger = HTTPEventLogger{
			Extractor: newProbeRequestLogger(),
			Logger:    eventLogger,
		}
		uiHTTPlogger = HTTPEventLogger{
			Extractor: newUIRequestLogger(userIDHeader),
			Logger:    eventLogger,
		}
		analyticsLogger = HTTPEventLogger{
			Extractor: newAnalyticsLogger(userIDHeader),
			Logger:    eventLogger,
		}
		webhooksLogger = HTTPEventLogger{
			Extractor: newWebhooksLogger(webhooks.WebhooksIntegrationTypeHeader),
			Logger:    eventLogger,
		}
	}

	authUserOrgDataAccessMiddleware := users_client.AuthOrgMiddleware{
		UsersClient: authenticator,
		OrgExternalID: func(r *http.Request) (string, bool) {
			v, ok := mux.Vars(r)["orgExternalID"]
			return v, ok
		},
		UserIDHeader:       userIDHeader,
		FeatureFlagsHeader: featureFlagsHeader,
		AuthorizeFor:       users.INSTANCE_DATA_ACCESS,
	}

	billingAuthMiddleware := users_client.AuthOrgMiddleware{
		UsersClient: authenticator,
		OrgExternalID: func(r *http.Request) (string, bool) {
			v, ok := mux.Vars(r)["orgExternalID"]
			return v, ok
		},
		UserIDHeader:        userIDHeader,
		FeatureFlagsHeader:  featureFlagsHeader,
		RequireFeatureFlags: []string{featureflag.Billing},
		AuthorizeFor:        users.OTHER,
	}

	authUserMiddleware := users_client.AuthUserMiddleware{
		UsersClient:  authenticator,
		UserIDHeader: userIDHeader,
	}

	webhooksMiddleware := users_client.WebhooksMiddleware{
		UsersClient:                   authenticator,
		WebhooksIntegrationTypeHeader: webhooks.WebhooksIntegrationTypeHeader,
	}

	fluxGHTokenMiddleware := users_client.GHIntegrationMiddleware{
		T: ghIntegration,
	}

	gcpWebhookSecretMiddleware := users_client.AuthSecretMiddleware{
		Secret: c.gcpWebhookSecret,
	}

	gcpLoginSecretMiddleware := users_client.GCPLoginSecretMiddleware{
		Secret: c.gcpSSOSecret,
	}

	scopeCensorMiddleware := users_client.ScopeCensorMiddleware{
		UsersClient:  authenticator,
		UserIDHeader: userIDHeader,
	}

	userPermissionsMiddleware := users_client.UserPermissionsMiddleware{
		UsersClient:  authenticator,
		UserIDHeader: userIDHeader,
		Permissions: []users_client.RequestPermission{
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
		},
	}

	// middleware to set header to disable caching if path == "/" exactly
	noCacheOnRoot := middleware.Func(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/" {
				w.Header().Set("Cache-Control", "no-cache")
			}
			next.ServeHTTP(w, r)
		})
	})

	r := newRouter()

	// Routes authenticated using header credentials
	dataUploadRoutes := MiddlewarePrefix{
		"/api",
		[]PrefixRoutable{
			Prefix{"/report", c.collectionHost},
			Prefix{"/prom/push", c.promDistributorHost},
			Prefix{"/net/peer", c.peerDiscoveryHost},
			PrefixMethods{"/flux", []string{"POST", "PATCH"}, c.fluxHost},
			PrefixMethods{"/prom/alertmanager/alerts", []string{"POST"}, c.promAlertmanagerHost},
			PrefixMethods{"/prom/alertmanager/v1/alerts", []string{"POST"}, c.promAlertmanagerHost},
			PrefixMethods{"/notification/external/events", []string{"POST"}, c.notificationEventHost},
		},
		middleware.Merge(
			users_client.AuthProbeMiddleware{
				UsersClient:        authenticator,
				FeatureFlagsHeader: featureFlagsHeader,
				AuthorizeFor:       users.INSTANCE_DATA_UPLOAD,
			},
			probeHTTPlogger,
		),
	}
	dataAccessRoutes := MiddlewarePrefix{
		"/api",
		Matchables([]Prefix{
			{"/control", c.controlHost},
			{"/pipe", c.pipeHost},
			{"/flux", c.fluxHost},
			{"/prom/alertmanager", c.promAlertmanagerHost},
			{"/prom/configs", c.promConfigsHost},
			{"/prom", c.promQuerierHost},
		}),
		middleware.Merge(
			users_client.AuthProbeMiddleware{
				UsersClient:        authenticator,
				FeatureFlagsHeader: featureFlagsHeader,
				AuthorizeFor:       users.INSTANCE_DATA_ACCESS,
			},
			probeHTTPlogger,
		),
	}

	// Internal version of the UI is served from the /internal/ prefix on ui-server
	var uiServerHandler http.Handler = c.uiServerHost
	if !c.externalUI {
		uiServerHandler = addPrefix("/internal", uiServerHandler)
	}

	for _, route := range []Routable{
		// Launcher service catch-all
		HostnameSpecific{
			c.launcherServiceExternalHost,
			[]PrefixRoutable{
				Prefix{"/", c.launcherServiceHost},
			},
			launcherServiceLogger,
		},

		// special case /demo redirect, which can't be inside a prefix{} rule as otherwise it matches /demo/
		Path{"/demo", redirect("/demo/")},

		// special case static version info
		Path{"/api", parseAPIInfo(c.apiInfo)},

		// For all ui <-> app communication, authenticated using cookie credentials
		MiddlewarePrefix{
			"/api/app/{orgExternalID}",
			[]PrefixRoutable{
				Prefix{"/api/report", scopeCensorMiddleware.Wrap(c.queryHost)},
				Prefix{"/api/topology", scopeCensorMiddleware.Wrap(c.queryHost)},
				Prefix{"/api/control", c.controlHost},
				Prefix{"/api/pipe", c.pipeHost},
				// API to insert deploy key requires GH token. Insert token with middleware.
				Prefix{"/api/flux/{flux_vsn}/integrations/github",
					fluxGHTokenMiddleware.Wrap(c.fluxHost)},
				// While we transition to newer Flux API
				Prefix{"/api/flux", c.fluxHost},
				Prefix{"/api/prom/alertmanager", c.promAlertmanagerHost},
				Prefix{"/api/prom/configs", c.promConfigsHost},
				Prefix{"/api/prom/notebooks", c.notebooksHost},
				Prefix{"/api/prom", c.promQuerierHost},
				Prefix{"/api/net/peer", c.peerDiscoveryHost},
				Prefix{"/api/notification/config", c.notificationEventHost},
				PrefixMethods{"/api/notification/events", []string{"GET"}, c.notificationEventHost},
				PrefixMethods{"/api/notification/events", []string{"POST"}, c.notificationEventHost},
				PrefixMethods{"/api/notification/testevent", []string{"POST"}, c.notificationEventHost},
				Prefix{"/api/notification/sender", c.notificationSenderHost},
				Prefix{"/api/gcp/users", c.gcpServiceHost},
				Prefix{"/api/dashboard", c.dashboardHost},
				Prefix{"/api", c.queryHost},

				// Catch-all forward to query service, which is a Scope instance that we
				// use to serve the Scope UI.  Note we forward /index.html to
				// /ui/index.html etc, as we never want to expose the root of a services
				// to the outside world - they can get at the debug info and metrics that
				// way.
				Prefix{"/", middleware.Merge(
					noCacheOnRoot,
					middleware.PathRewrite(regexp.MustCompile("(.*)"), "/ui$1"),
				).Wrap(c.queryHost)},
			},
			middleware.Merge(
				authUserOrgDataAccessMiddleware,
				userPermissionsMiddleware,
				middleware.PathRewrite(regexp.MustCompile("^/api/app/[^/]+"), ""),
				uiHTTPlogger,
			),
		},

		// The billing API requires authentication & authorization.
		MiddlewarePrefix{
			"/api/billing/{orgExternalID}",
			[]PrefixRoutable{
				Prefix{"/", c.billingAPIHost},
			},
			middleware.Merge(
				billingAuthMiddleware,
				uiHTTPlogger,
			),
		},

		// Google Single Sign-On for GCP Cloud Launcher integration.
		MiddlewarePrefix{
			"/api/users/gcp/sso/login",
			[]PrefixRoutable{
				Prefix{"/", c.usersHost},
			},
			middleware.Merge(
				gcpLoginSecretMiddleware,
				uiHTTPlogger,
			),
		},

		// Webhook for events from Google PubSub require authentication through a secret.
		MiddlewarePrefix{
			"/api/gcp-launcher/webhook",
			[]PrefixRoutable{
				Prefix{"/", c.gcpWebhookHost},
			},
			middleware.Merge(
				gcpWebhookSecretMiddleware,
				middleware.PathRewrite(regexp.MustCompile("(.*)"), ""),
				uiHTTPlogger,
			),
		},

		// GCP billing integration
		// Redirect POST requests to /subscribe-via/gcp and /login-via/gcp coming from GCP Marketplace to GET requests
		// which can be handled by the frontend
		PathMethods{
			"/{action:subscribe|login}-via/gcp",
			[]string{http.MethodPost},
			redirectWithQuery,
		},
		Path{
			"/api/ui/analytics",
			middleware.Merge(authUserMiddleware, analyticsLogger).Wrap(noopHandler),
		},

		MiddlewarePrefix{
			"/api/flux/{flux_vsn}/integrations",
			[]PrefixRoutable{
				PrefixMethods{"/dockerhub/image", []string{"POST"}, c.fluxHost},
				PrefixMethods{"/quay/image", []string{"POST"}, c.fluxHost},
			},
			nil,
		},

		// Webhooks
		Path{
			"/webhooks/{secretID}/",
			middleware.Merge(webhooksLogger, webhooksMiddleware).Wrap(c.fluxHost),
		},

		// Token-based auth
		dataUploadRoutes,
		dataAccessRoutes,

		// Unauthenticated webhook for Atlantis. We trust Atlantis to do its own authenication.
		MiddlewarePrefix{
			"/",
			Matchables([]Prefix{
				{"/admin/corp-atlantis/events", trimPrefix("/admin/corp-atlantis", c.corpAtlantisHost)},
			}),
			uiHTTPlogger,
		},

		// For all admin functionality, authenticated using header credentials
		MiddlewarePrefix{
			"/admin",
			Matchables([]Prefix{
				{"/grafana", trimPrefix("/admin/grafana", c.grafanaHost)},
				{"/dev-grafana", trimPrefix("/admin/dev-grafana", c.devGrafanaHost)},
				{"/prod-grafana", trimPrefix("/admin/prod-grafana", c.prodGrafanaHost)},
				{"/scope", trimPrefix("/admin/scope", c.scopeHost)},
				{"/users", c.usersHost},
				{"/billing.csv", c.billingAPIHost},
				{"/billing/organizations", trimPrefix("/admin/billing/organizations", c.billingAPIHost)},
				{"/billing/aggregator", trimPrefix("/admin/billing/aggregator", c.billingAggregatorHost)},
				{"/billing/enforcer", trimPrefix("/admin/billing/enforcer", c.billingEnforcerHost)},
				{"/billing/uploader", trimPrefix("/admin/billing/uploader", c.billingUploaderHost)},
				{"/billing/invoice-verify", c.billingAPIHost},
				{"/kubediff", trimPrefix("/admin/kubediff", c.kubediffHost)},
				{"/terradiff", trimPrefix("/admin/terradiff", c.terradiffHost)},
				{"/ansiblediff", trimPrefix("/admin/ansiblediff", c.ansiblediffHost)},
				{"/alertmanager", c.alertmanagerHost},
				{"/prometheus", c.prometheusHost},
				{"/kubedash", trimPrefix("/admin/kubedash", c.kubedashHost)},
				{"/compare-images", trimPrefix("/admin/compare-images", c.compareImagesHost)},
				{"/compare-revisions", trimPrefix("/admin/compare-revisions", c.compareRevisionsHost)},
				{"/cortex/alertmanager/status", trimPrefix("/admin/cortex/alertmanager", c.promAlertmanagerHost)},
				{"/cortex/ring", trimPrefix("/admin/cortex", c.promDistributorHost)},
				{"/cortex/all_user_stats", trimPrefix("/admin/cortex", c.promDistributorHost)},
				{"/loki", trimPrefix("/admin/loki", c.lokiHost)},
				{"/jaeger", c.jaegerHost},
				{"/kibana", trimPrefix("/admin/kibana", c.kibanaHost)},
				{"/elasticsearch", trimPrefix("/admin/elasticsearch", c.elasticsearchHost)},
				{"/esh", trimPrefix("/admin/esh", c.eshHost)},
				{"/corp-atlantis", trimPrefix("/admin/corp-atlantis", c.corpAtlantisHost)},
				{"/corp-terradiff", trimPrefix("/admin/corp-terradiff", c.corpTerradiffHost)},
				{"/", http.HandlerFunc(adminRoot)},
			}),
			middleware.Merge(
				// If not logged in, prompt user to log in instead of 401ing
				middleware.ErrorHandler{
					Code: 401,
					Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						q := make(url.Values)
						q.Set("next", r.RequestURI)
						http.Redirect(w, r, fmt.Sprintf("/login?%s", q.Encode()), 302)
					}),
				},
				users_client.AuthAdminMiddleware{
					UsersClient: authenticator,
				},
			),
		},

		// unauthenticated communication
		MiddlewarePrefix{
			"/",
			Matchables([]Prefix{
				// Users service does its own auth.
				{"/api/users", c.usersHost},

				// Launch generator routes.
				{"/launch/k8s", c.launchGeneratorHost},
				{"/k8s", c.launchGeneratorHost},

				// Demo service paths get rewritten to remove /demo/ prefix.
				{"/demo", middleware.PathRewrite(regexp.MustCompile("/demo/(.*)"), "/$1").Wrap(c.demoHost)},

				// Forward requests (unauthenticated) to the ui-metrics job.
				{"/api/ui/metrics", c.uiMetricsHost},

				// Forward requests to the service-ui-kicker job.
				// There is authentication done inside service-ui-kicker itself to ensure requests came from github
				{"/service-ui-kicker", c.serviceUIKickerHost},

				// Final wildcard match to static content
				{"/", noCacheOnRoot.Wrap(uiServerHandler)},
			}),
			uiHTTPlogger,
		},
	} {
		route.RegisterRoutes(r)
	}

	// Do not check for csrf tokens in requests from:
	// * probes (they cannot be attacked)
	// * the admin alert manager, incorporating tokens would require forking it
	//   and we don't see alert-silencing as very security-sensitive.
	// * the Cortex alert manager, incorporating tokens would require forking it
	//   (see https://github.com/weaveworks/service-ui/issues/461#issuecomment-299458350)
	//   and we don't see alert-silencing as very security-sensitive.
	// * incoming webhooks:
	//   - service-ui-kicker
	//   - gcp-launcher-webhook
	//   - gcp subscribe/login redirects
	//   - dockerhub
	//   - corp-atlantis
	//   as these are validated by checking hmac integrity or arbitrary secrets.
	csrfExemptPrefixes := dataUploadRoutes.AbsolutePrefixes()
	csrfExemptPrefixes = append(csrfExemptPrefixes, dataAccessRoutes.AbsolutePrefixes()...)
	csrfExemptPrefixes = append(
		csrfExemptPrefixes,
		"/admin/alertmanager",
		"/service-ui-kicker",
		"/api/ui/metrics",
		"/api/gcp-launcher/webhook",
		"/(subscribe|login)-via/gcp",
		`/api/app/[a-zA-Z0-9_-]+/api/prom/alertmanager`, // Regex copy-pasted from users/organization.go
		"/api/users/signup_webhook",                     // Validated by explicit token in the users service
		"/api/users/org/platform_version",               // Also validated by explicit token
		"/webhooks",                                     // POSTed to by external services

		"/admin/corp-atlantis", // GitHub webhook (/events) & discarding locks (/locks)
		"/admin/grafana",       // grafana does not know to inject CSRF header into requests. Without whitelisting,
		"/admin/dev-grafana",   // this causes the CSRF token & cookie to be re-issued, breaking UI requests.
		"/admin/prod-grafana",
		"/admin/kibana", // kibana has the same issue with CSRF tokens as grafana
	)

	// Strip csrf_token cookies set by the nosurf middleware because the nosurf
	// middleware does not allow us to exclude paths for setting cookies, only
	// exclude them from being validated. Strip all cookies because parsing
	// already set 'Set-Cookie' headers is more involved.
	stripSetCookieHeaderPrefixes := []string{
		"/admin/grafana",
		"/admin/dev-grafana",
		"/admin/prod-grafana",
		"/admin/kibana",
	}

	operationNameFunc := nethttp.OperationNameFunc(func(r *http.Request) string {
		return fmt.Sprintf("%s %s", r.Method, r.URL.Path)
	})
	originChecker := originCheckerMiddleware{
		allowedOrigin:         c.targetOrigin,
		allowedOriginSuffixes: c.allowedOriginSuffixes,
	}
	return middleware.Merge(
		AuthHeaderStrippingMiddleware{},
		originChecker,
		stripSetCookieHeader{prefixes: stripSetCookieHeaderPrefixes},
		csrfTokenVerifier{exemptPrefixes: csrfExemptPrefixes, secure: c.secureCookie, domain: c.cookieDomain},
		middleware.Func(func(handler http.Handler) http.Handler {
			return nethttp.Middleware(opentracing.GlobalTracer(), handler, operationNameFunc)
		}),
		c.commonMiddleWare(r),
	).Wrap(r), nil
}

// stripSetCookieHeader deletes any "Set-Cookie" header where the path matches a prefix
type stripSetCookieHeader struct {
	prefixes []string
}

func (s stripSetCookieHeader) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		match := false
		for _, prefix := range s.prefixes {
			if strings.HasPrefix(r.URL.Path, prefix) {
				match = true
				break
			}
		}
		if !match {
			next.ServeHTTP(w, r)
			return
		}

		// If we need to modify the response (headers or body)
		// we need to intercept it like this, because the reverse proxy
		// sends the response before control returns to the middleware.

		rec := httptest.NewRecorder()
		next.ServeHTTP(rec, r)
		resp := rec.Result()

		responseHeader := w.Header()
		headers := resp.Header
		// Remove the cookie
		headers.Del("Set-Cookie")
		// Copy the original headers
		for k, v := range headers {
			responseHeader[k] = v
		}

		// Finally, write the response
		w.WriteHeader(resp.StatusCode)
		body, _ := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		w.Write(body)
	})
}

// Takes care of setting and verifying anti-CSRF tokens
// * Injects the CSRF token in html responses, by replacing the the $__CSRF_TOKEN_PLACEHOLDER__ string
//   (so that the JS code can include it in all its requests).
// * Sets the CSRF token in cookie.
// * Checks them against each other.
// It complements static origin checking
type csrfTokenVerifier struct {
	exemptPrefixes []string
	secure         bool
	domain         string
}

func (c csrfTokenVerifier) Wrap(next http.Handler) http.Handler {
	h := nosurf.New(injectTokenInHTMLResponses(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// NoSurf might have parsed the body for us already; if so,
		// copy that back into Body.
		if r.PostForm != nil {
			r.Body = ioutil.NopCloser(strings.NewReader(r.PostForm.Encode()))
		}

		next.ServeHTTP(w, r)
	})))
	h.SetBaseCookie(http.Cookie{
		MaxAge:   nosurf.MaxAge,
		HttpOnly: true,
		Path:     "/",
		Secure:   c.secure,
		Domain:   c.domain,
	})
	// Make errors a bit more descriptive than a plain 400
	h.SetFailureHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "CSRF token mismatch: "+nosurf.Reason(r).Error(), http.StatusBadRequest)
	}))
	for _, prefix := range c.exemptPrefixes {
		h.ExemptRegexp(fmt.Sprintf("^%s.*$", prefix))
	}
	return h
}

func injectTokenInHTMLResponses(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only consider non-websocket GET methods
		if r.Method != http.MethodGet || middleware.IsWSHandshakeRequest(r) {
			next.ServeHTTP(w, r)
			return
		}

		rec := httptest.NewRecorder()
		next.ServeHTTP(rec, r)
		resp := rec.Result()

		responseHeader := w.Header()
		// Copy the original headers
		for k, v := range resp.Header {
			responseHeader[k] = v
		}

		responseBody, _ := ioutil.ReadAll(resp.Body)
		resp.Body.Close()

		if mtype, _, err := mime.ParseMediaType(responseHeader.Get("Content-Type")); err == nil && mtype == "text/html" {
			responseBody = bytes.Replace(responseBody, []byte("$__CSRF_TOKEN_PLACEHOLDER__"), []byte(nosurf.Token(r)), -1)
			// Adjust content length if present
			if responseHeader.Get("Content-Length") != "" {
				responseHeader.Set("Content-Length", strconv.Itoa(len(responseBody)))
			}
			// Disable caching. The token needs to be reloaded every
			// time the cookie changes.
			responseHeader.Set("Cache-Control", "no-cache, no-store, must-revalidate")
			responseHeader.Set("Pragma", "no-cache")
			responseHeader.Set("Expires", "0")
		}

		// Finally, write the response
		w.WriteHeader(resp.StatusCode)
		w.Write(responseBody)
	})
}

// Checks Origin and Referer headers against the expected target of the site
// to protect against CSRF attacks.
type originCheckerMiddleware struct {
	allowedOrigin         string   // The expected origin under normal operation
	allowedOriginSuffixes []string // permitted origin suffixes e.g. .build.dev.weave.works
}

func (o originCheckerMiddleware) Wrap(next http.Handler) http.Handler {
	if o.allowedOrigin == "" && len(o.allowedOriginSuffixes) == 0 {
		// Nothing to check against
		return next
	}

	headerMatchesTarget := func(headerName string, r *http.Request, logger logging.Interface) bool {
		if headerValue := r.Header.Get(headerName); headerValue != "" {
			url, err := url.Parse(headerValue)
			if err != nil {
				logger.Warnf("originCheckerMiddleware: Cannot parse %s header: %v", headerName, err)
				return false
			}
			if o.allowedOrigin != "" && url.Host == o.allowedOrigin {
				return true
			}
			if o.allowedOriginSuffixes != nil {
				for _, suffix := range o.allowedOriginSuffixes {
					if strings.HasSuffix(url.Host, suffix) {
						return true
					}
				}
			}
			return false
		}
		// If the header is missing we intentionally consider it a match
		// Some legitimate requests come without headers, e.g: Scope probe requests and non-js browser requests
		return true
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := user.LogWith(r.Context(), logging.Global())
		// Verify that origin or referer headers (when present) match the expected target
		permitted := isSafeMethod(r.Method) || (headerMatchesTarget("Origin", r, logger) && headerMatchesTarget("Referer", r, logger))

		logger.WithFields(
			logging.Fields{
				"URL":                   r.URL,
				"Method":                r.Method,
				"Origin":                r.Header.Get("Origin"),
				"Referer":               r.Referer(),
				"allowedOrigin":         o.allowedOrigin,
				"allowedOriginSuffixes": o.allowedOriginSuffixes,
				"Permitted":             permitted,
			}).Debugf("originCheckerMiddleware checked request")

		if permitted {
			next.ServeHTTP(w, r)
		} else {
			w.WriteHeader(http.StatusForbidden)
		}
	})
}

// AuthHeaderStrippingMiddleware strips weaveworks auth headers from incoming http requests to prevent impersonation
// The users/client middleware also checks these headers, and prevents double headers and impersonation by raising an
// error. However, internal grafana needs to proxy requests for flux data to cloud.w.w, but this is disallowed due to
// double header errors. We concluded it is simpler & safer to drop headers at authfe, than in the common middleware.
//
// see https://github.com/weaveworks/common/pull/63 & https://github.com/weaveworks/service-conf/pull/1295
type AuthHeaderStrippingMiddleware struct {
}

// Wrap another HTTP handler
func (a AuthHeaderStrippingMiddleware) Wrap(next http.Handler) http.Handler {
	removeHeader := func(headerName string, r *http.Request) {
		value := r.Header.Get(headerName)
		if value != "" {
			user.LogWith(r.Context(), logging.Global()).Debugf("AuthHeaderStrippingMiddleware: Stripped auth header from incoming request (%s: %s) URL: %s Referer: %s",
				headerName, value, r.URL, r.Referer())
			r.Header.Del(headerName)
		}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		removeHeader(user.OrgIDHeaderName, r)
		removeHeader(user.UserIDHeaderName, r)
		next.ServeHTTP(w, r)
	})
}

func isSafeMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodHead
}

// Gorilla Router with sensible defaults, namely:
// - StrictSlash set to false
// - SkipClean set to true
//
// This allows for /foo/bar/%2fbaz%2fqux URLs to be forwarded correctly.
func newRouter() *mux.Router {
	return mux.NewRouter().StrictSlash(false).SkipClean(true)
}

func trimPrefix(regex string, handler http.Handler) http.Handler {
	return middleware.PathRewrite(regexp.MustCompile("^"+regex), "").Wrap(handler)
}

func addPrefix(prefix string, handler http.Handler) http.Handler {
	return middleware.PathRewrite(regexp.MustCompile("^"), prefix).Wrap(handler)
}

func redirect(dest string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, dest, 302)
	})
}

var redirectWithQuery = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	logger := logging.WithField("url", r.URL.Path)
	logger.Infoln("Handling GCP integration request")

	if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	// Limit request body to 16KiB
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := r.ParseForm(); err != nil {
		logger.Errorf("Error parsing form: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	gcpToken := r.PostForm.Get(gcp.MarketplaceTokenParam)
	if gcpToken == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	query := r.URL.Query()
	query.Add(gcp.MarketplaceTokenParam, gcpToken)
	redirectURL := fmt.Sprintf("%s?%s", r.URL.Path, query.Encode())
	http.Redirect(w, r, redirectURL, http.StatusFound)
})
