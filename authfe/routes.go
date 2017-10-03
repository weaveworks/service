package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"github.com/justinas/nosurf"
	"github.com/opentracing-contrib/go-stdlib/nethttp"
	"github.com/opentracing/opentracing-go"

	"github.com/weaveworks/common/logging"
	"github.com/weaveworks/common/middleware"
	"github.com/weaveworks/common/user"
	"github.com/weaveworks/scope/common/xfer"
	"github.com/weaveworks/service/common"
	"github.com/weaveworks/service/users"
	users_client "github.com/weaveworks/service/users/client"
)

const maxAnalyticsPayloadSize = 16 * 1024 // bytes

func (c Config) commonMiddleWare(routeMatcher middleware.RouteMatcher) middleware.Interface {
	extraHeaders := http.Header{}
	extraHeaders.Add("X-Frame-Options", "SAMEORIGIN")
	extraHeaders.Add("X-XSS-Protection", "1; mode=block")
	extraHeaders.Add("X-Content-Type-Options", "nosniff")
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
		middleware.Log{},
	)
}

var noopHandler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNoContent)
})

func mustSplitHostname(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		logging.With(r.Context()).Errorf("Error splitting '%s': %v", r.RemoteAddr, err)
	}
	return host
}

// ifEmpty(a,b) returns b iff a is empty
func ifEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func newProbeRequestLogger() HTTPEventExtractor {
	return func(r *http.Request) (Event, bool) {
		orgID, err := user.ExtractOrgID(r.Context())
		if err != nil {
			return Event{}, false
		}

		event := Event{
			ID:             r.URL.Path,
			Product:        "scope-probe",
			Version:        r.Header.Get(xfer.ScopeProbeVersionHeader),
			UserAgent:      r.UserAgent(),
			ClientID:       r.Header.Get(xfer.ScopeProbeIDHeader),
			OrganizationID: orgID,
			IPAddress:      mustSplitHostname(r),
		}
		return event, true
	}
}

func newUIRequestLogger(userIDHeader string) HTTPEventExtractor {
	return func(r *http.Request) (Event, bool) {
		sessionCookie, err := r.Cookie(sessionCookieKey)
		var sessionID string
		if err == nil {
			sessionID = sessionCookie.Value
		}

		orgID, _ := user.ExtractOrgID(r.Context())

		event := Event{
			ID:             r.URL.Path,
			SessionID:      sessionID,
			Product:        "scope-ui",
			UserAgent:      r.UserAgent(),
			OrganizationID: orgID,
			UserID:         r.Header.Get(userIDHeader),
			IPAddress:      mustSplitHostname(r),
		}
		return event, true
	}
}

func newAnalyticsLogger(userIDHeader string) HTTPEventExtractor {
	return func(r *http.Request) (Event, bool) {
		sessionCookie, err := r.Cookie(sessionCookieKey)
		var sessionID string
		if err == nil {
			sessionID = sessionCookie.Value
		}

		values, err := ioutil.ReadAll(&io.LimitedReader{
			R: r.Body,
			N: maxAnalyticsPayloadSize,
		})
		if err != nil {
			return Event{}, false
		}

		event := Event{
			ID:        r.URL.Path,
			SessionID: sessionID,
			Product:   "scope-ui",
			UserAgent: r.UserAgent(),
			UserID:    r.Header.Get(userIDHeader),
			Values:    string(values),
			IPAddress: mustSplitHostname(r),
		}
		return event, true
	}
}

func routes(c Config, authenticator users.UsersClient, ghIntegration *users_client.TokenRequester, eventLogger *EventLogger) (http.Handler, error) {
	probeHTTPlogger, uiHTTPlogger, analyticsLogger := middleware.Identity, middleware.Identity, middleware.Identity
	if eventLogger != nil {
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
		RequireFeatureFlags: []string{"billing"},
		AuthorizeFor:        users.OTHER,
	}

	authUserMiddleware := users_client.AuthUserMiddleware{
		UsersClient:  authenticator,
		UserIDHeader: userIDHeader,
	}

	fluxGHTokenMiddleware := users_client.GHIntegrationMiddleware{
		T: ghIntegration,
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
			PrefixMethods{"/flux/{flux_vsn:v[345]}", []string{"POST", "PATCH"}, c.fluxHost},
			PrefixMethods{"/flux", []string{"POST", "PATCH"}, c.fluxV6Host},
			PrefixMethods{"/prom/alertmanager/alerts", []string{"POST"}, c.promAlertmanagerHost},
			PrefixMethods{"/prom/alertmanager/v1/alerts", []string{"POST"}, c.promAlertmanagerHost},
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
			{"/flux/{flux_vsn:v[345]}", c.fluxHost},
			{"/flux", c.fluxV6Host},
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
		// special case /demo redirect, which can't be inside a prefix{} rule as otherwise it matches /demo/
		Path{"/demo", redirect("/demo/")},

		// special case static version info
		Path{"/api", parseAPIInfo(c.apiInfo)},

		// For all ui <-> app communication, authenticated using cookie credentials
		MiddlewarePrefix{
			"/api/app/{orgExternalID}",
			[]PrefixRoutable{
				Prefix{"/api/report", c.queryHost},
				Prefix{"/api/topology", c.queryHost},
				Prefix{"/api/control", c.controlHost},
				Prefix{"/api/pipe", c.pipeHost},
				// API to insert deploy key requires GH token. Insert token with middleware.
				Prefix{"/api/flux/v5/integrations/github",
					fluxGHTokenMiddleware.Wrap(c.fluxHost)},
				Prefix{"/api/flux/{flux_vsn}/integrations/github",
					fluxGHTokenMiddleware.Wrap(c.fluxV6Host)},
				// While we transition to newer Flux API
				Prefix{"/api/flux/{flux_vsn:v[345]}", c.fluxHost},
				Prefix{"/api/flux", c.fluxV6Host},
				Prefix{"/api/prom/alertmanager", c.promAlertmanagerHost},
				Prefix{"/api/prom/configs", c.promConfigsHost},
				Prefix{"/api/prom/notebooks", c.notebooksHost},
				Prefix{"/api/prom", c.promQuerierHost},
				Prefix{"/api/net/peer", c.peerDiscoveryHost},
				Prefix{"/api/notification/config", c.notificationConfigHost},
				PrefixMethods{"/api/notification/events", []string{"GET"}, c.notificationConfigHost},
				Prefix{"/api/notification/sender", c.notificationSenderHost},
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

		Path{
			"/api/ui/analytics",
			middleware.Merge(authUserMiddleware, analyticsLogger).Wrap(noopHandler),
		},

		// Token-based auth
		dataUploadRoutes,
		dataAccessRoutes,

		// For all admin functionality, authenticated using header credentials
		MiddlewarePrefix{
			"/admin",
			Matchables([]Prefix{
				{"/grafana", trimPrefix("/admin/grafana", c.grafanaHost)},
				{"/dev-grafana", trimPrefix("/admin/dev-grafana", c.devGrafanaHost)},
				{"/prod-grafana", trimPrefix("/admin/prod-grafana", c.prodGrafanaHost)},
				{"/scope", trimPrefix("/admin/scope", c.scopeHost)},
				{"/users", c.usersHost},
				{"/billing/organizations", trimPrefix("/admin/billing/organizations", c.billingAPIHost)},
				{"/billing/aggregator", trimPrefix("/admin/billing/aggregator", c.billingAggregatorHost)},
				{"/billing/enforcer", trimPrefix("/admin/billing/enforcer", c.billingEnforcerHost)},
				{"/billing/uploader", trimPrefix("/admin/billing/uploader", c.billingUploaderHost)},
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
				{"/loki", trimPrefix("/admin/loki", c.lokiHost)},
				{"/", http.HandlerFunc(adminRoot)},
			}),
			middleware.Merge(
				// If not logged in, prompt user to log in instead of 401ing
				middleware.ErrorHandler{
					Code:    401,
					Handler: redirect("/login"),
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

				// Forward Github WebHooks to github-receiver.
				// There is authentication done inside github-receiver itself to ensure requests came from github
				{"/github-receiver", c.githubReceiverHost},

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
	// * incoming webhooks (service-ui-kicker and github-receiver), as these are validated
	//   by checking HMAC integrity
	csrfExemptPrefixes := dataUploadRoutes.AbsolutePrefixes()
	csrfExemptPrefixes = append(csrfExemptPrefixes, dataAccessRoutes.AbsolutePrefixes()...)
	csrfExemptPrefixes = append(
		csrfExemptPrefixes,
		"/admin/alertmanager",
		"/service-ui-kicker",
		"/api/ui/metrics",
		"/github-receiver",
		`/api/app/[a-zA-Z0-9_-]+/api/prom/alertmanager`, // Regex copy-pasted from users/organization.go
		"/api/users/signup_webhook",                     // Validated by explicit token in the users service
	)
	return middleware.Merge(
		AuthHeaderStrippingMiddleware{},
		originCheckerMiddleware{expectedTarget: c.targetOrigin},
		csrfTokenVerifier{exemptPrefixes: csrfExemptPrefixes, secure: c.secureCookie},
		middleware.Func(func(handler http.Handler) http.Handler {
			return nethttp.Middleware(opentracing.GlobalTracer(), handler)
		}),
		c.commonMiddleWare(r),
	).Wrap(r), nil
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
		responseHeader := w.Header()

		// Copy the original headers
		for k, v := range rec.Header() {
			responseHeader[k] = v
		}

		responseBody := rec.Body.Bytes()

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
		w.WriteHeader(rec.Code)
		w.Write(responseBody)
	})
}

// Checks Origin and Referer headers against the expected target of the site
// to protect against CSRF attacks.
type originCheckerMiddleware struct {
	expectedTarget string
}

func (o originCheckerMiddleware) Wrap(next http.Handler) http.Handler {
	if o.expectedTarget == "" {
		// Nothing to check against
		return next
	}

	headerMatchesTarget := func(headerName string, r *http.Request) bool {
		if headerValue := r.Header.Get(headerName); headerValue != "" {
			url, err := url.Parse(headerValue)
			if err != nil {
				logging.With(r.Context()).Warnf("originCheckerMiddleware: Cannot parse %s header: %v", headerName, err)
				return false
			}
			return url.Host == o.expectedTarget
		}
		// If the header is missing we intentionally consider it a match
		// Some legitimate requests come without headers, e.g: Scope probe requests and non-js browser requests
		return true
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logging.With(r.Context()).Debugf("originCheckerMiddleware: URL %s, Method %s, Origin: %q, Referer: %q, expectedTarget: %q",
			r.URL, r.Method, r.Header.Get("Origin"), r.Referer(), o.expectedTarget)

		// Verify that origin or referer headers (when present) match the expected target
		if !isSafeMethod(r.Method) && (!headerMatchesTarget("Origin", r) || !headerMatchesTarget("Referer", r)) {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
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
			logging.With(r.Context()).Debugf("AuthHeaderStrippingMiddleware: Stripped auth header from incoming request (%s: %s) URL: %s Referer: %s",
				headerName, value, r.URL, r.Referer())
			r.Header.Del(headerName)
		}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		removeHeader("X-Scope-OrgID", r)
		removeHeader("X-Scope-UserID", r)
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
