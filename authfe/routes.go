package main

import (
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"

	log "github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	"github.com/opentracing-contrib/go-stdlib/nethttp"
	"github.com/opentracing/opentracing-go"

	"github.com/weaveworks/common/httpgrpc"
	"github.com/weaveworks/common/middleware"
	"github.com/weaveworks/scope/common/xfer"
	"github.com/weaveworks/service/common"
	users "github.com/weaveworks/service/users/client"
)

const maxAnalyticsPayloadSize = 16 * 1024 // bytes

// Config is all the config we need to build the routes
type Config struct {
	authenticator users.Authenticator
	eventLogger   *EventLogger
	outputHeader  string
	logSuccess    bool
	apiInfo       string
	targetOrigin  string

	// User-visible services - keep alphabetically sorted pls
	collectionHost      string
	queryHost           string
	controlHost         string
	pipeHost            string
	fluxHost            string
	configsHost         string
	billingUIHost       string
	billingAPIHost      string
	billingUsageHost    string
	demoHost            string
	launchGeneratorHost string
	uiMetricsHost       string
	uiServerHost        string

	// Admin services - keep alphabetically sorted pls
	alertmanagerHost        string
	ansiblediffHost         string
	devGrafanaHost          string
	compareImagesHost       string
	grafanaHost             string
	kubedashHost            string
	kubediffHost            string
	lokiHost                string
	prodGrafanaHost         string
	promDistributorHost     string
	promDistributorHostGRPC string
	prometheusHost          string
	promQuerierHost         string
	promQuerierHostGRPC     string
	scopeHost               string
	terradiffHost           string
	usersHost               string
}

var noopHandler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNoContent)
})

func mustSplitHostname(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		log.Errorf("Error splitting '%s': %v", r.RemoteAddr, err)
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

func newProbeRequestLogger(orgIDHeader string) HTTPEventExtractor {
	return func(r *http.Request) (Event, bool) {

		event := Event{
			ID:             r.URL.Path,
			Product:        "scope-probe",
			Version:        r.Header.Get(xfer.ScopeProbeVersionHeader),
			UserAgent:      r.UserAgent(),
			ClientID:       r.Header.Get(xfer.ScopeProbeIDHeader),
			OrganizationID: r.Header.Get(orgIDHeader),
			IPAddress:      mustSplitHostname(r),
		}
		return event, true
	}
}

func newUIRequestLogger(orgIDHeader, userIDHeader string) HTTPEventExtractor {
	return func(r *http.Request) (Event, bool) {
		sessionCookie, err := r.Cookie(sessionCookieKey)
		var sessionID string
		if err == nil {
			sessionID = sessionCookie.Value
		}

		event := Event{
			ID:             r.URL.Path,
			SessionID:      sessionID,
			Product:        "scope-ui",
			UserAgent:      r.UserAgent(),
			OrganizationID: r.Header.Get(orgIDHeader),
			UserID:         r.Header.Get(userIDHeader),
			IPAddress:      mustSplitHostname(r),
		}
		return event, true
	}
}

func newAnalyticsLogger(orgIDHeader, userIDHeader string) HTTPEventExtractor {
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
			ID:             r.URL.Path,
			SessionID:      sessionID,
			Product:        "scope-ui",
			UserAgent:      r.UserAgent(),
			OrganizationID: r.Header.Get(orgIDHeader),
			UserID:         r.Header.Get(userIDHeader),
			Values:         string(values),
			IPAddress:      mustSplitHostname(r),
		}
		return event, true
	}
}

func routes(c Config) (http.Handler, error) {
	probeHTTPlogger, uiHTTPlogger, analyticsLogger := middleware.Identity, middleware.Identity, middleware.Identity
	if c.eventLogger != nil {
		probeHTTPlogger = HTTPEventLogger{
			Extractor: newProbeRequestLogger(c.outputHeader),
			Logger:    c.eventLogger,
		}
		uiHTTPlogger = HTTPEventLogger{
			Extractor: newUIRequestLogger(c.outputHeader, userIDHeader),
			Logger:    c.eventLogger,
		}
		analyticsLogger = HTTPEventLogger{
			Extractor: newAnalyticsLogger(c.outputHeader, userIDHeader),
			Logger:    c.eventLogger,
		}
	}

	authOrgMiddleware := users.AuthOrgMiddleware{
		Authenticator: c.authenticator,
		OrgExternalID: func(r *http.Request) (string, bool) {
			v, ok := mux.Vars(r)["orgExternalID"]
			return v, ok
		},
		OutputHeader:       c.outputHeader,
		UserIDHeader:       userIDHeader,
		FeatureFlagsHeader: featureFlagsHeader,
	}

	authUserMiddleware := users.AuthUserMiddleware{
		Authenticator: c.authenticator,
		UserIDHeader:  userIDHeader,
	}

	billingAuthMiddleware := users.AuthOrgMiddleware{
		Authenticator: c.authenticator,
		OrgExternalID: func(r *http.Request) (string, bool) {
			v, ok := mux.Vars(r)["orgExternalID"]
			return v, ok
		},
		OutputHeader:        c.outputHeader,
		UserIDHeader:        userIDHeader,
		FeatureFlagsHeader:  featureFlagsHeader,
		RequireFeatureFlags: []string{"billing"},
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

	var cortexQuerierClient, cortexDistributorClient http.Handler
	if c.promQuerierHostGRPC == "" {
		cortexQuerierClient = newProxy(c.promQuerierHost)
	} else {
		var err error
		cortexQuerierClient, err = httpgrpc.NewClient(c.promQuerierHostGRPC)
		if err != nil {
			return nil, err
		}
	}

	if c.promDistributorHostGRPC == "" {
		cortexDistributorClient = newProxy(c.promDistributorHost)
	} else {
		var err error
		cortexDistributorClient, err = httpgrpc.NewClient(c.promDistributorHostGRPC)
		if err != nil {
			return nil, err
		}
	}

	r := newRouter()
	for _, route := range []routable{
		// demo service paths get rewritten to remove /demo/ prefix, so trailing slash is required
		path{"/demo", redirect("/demo/")},
		// special case static version info
		path{"/api", parseAPIInfo(c.apiInfo)},

		// For all ui <-> app communication, authenticated using cookie credentials
		prefix{
			"/api/app/{orgExternalID}",
			[]path{
				{"/api/report", newProxy(c.queryHost)},
				{"/api/topology", newProxy(c.queryHost)},
				{"/api/control", newProxy(c.controlHost)},
				{"/api/pipe", newProxy(c.pipeHost)},
				{"/api/configs", newProxy(c.configsHost)},
				{"/api/flux", newProxy(c.fluxHost)},
				{"/api/prom", cortexQuerierClient},
				{"/api", newProxy(c.queryHost)},

				// Catch-all forward to query service, which is a Scope instance that we
				// use to serve the Scope UI.  Note we forward /index.html to
				// /ui/index.html etc, as we never want to expose the root of a services
				// to the outside world - they can get at the debug info and metrics that
				// way.
				{"/", middleware.Merge(
					noCacheOnRoot,
					middleware.PathRewrite(regexp.MustCompile("(.*)"), "/ui$1"),
				).Wrap(newProxy(c.queryHost))},
			},
			middleware.Merge(
				authOrgMiddleware,
				middleware.PathRewrite(regexp.MustCompile("^/api/app/[^/]+"), ""),
				uiHTTPlogger,
			),
		},

		// Forward requests (unauthenticated) to the ui-metrics job.
		path{
			"/api/ui/metrics",
			newProxy(c.uiMetricsHost),
		},

		path{
			"/api/ui/analytics",
			middleware.Merge(authUserMiddleware, analyticsLogger).Wrap(noopHandler),
		},

		// For all probe <-> app communication, authenticated using header credentials
		prefix{
			"/api",
			[]path{
				{"/report", newProxy(c.collectionHost)},
				{"/control", newProxy(c.controlHost)},
				{"/pipe", newProxy(c.pipeHost)},
				{"/configs", newProxy(c.configsHost)},
				{"/flux", newProxy(c.fluxHost)},
				{"/prom/push", cortexDistributorClient},
				{"/prom", cortexQuerierClient},
			},
			middleware.Merge(
				users.AuthProbeMiddleware{
					Authenticator:      c.authenticator,
					OutputHeader:       c.outputHeader,
					FeatureFlagsHeader: featureFlagsHeader,
				},
				probeHTTPlogger,
			),
		},

		// For all admin functionality, authenticated using header credentials
		prefix{
			"/admin",
			[]path{
				{"/grafana", trimPrefix("/admin/grafana", newProxy(c.grafanaHost))},
				{"/dev-grafana", trimPrefix("/admin/dev-grafana", newProxy(c.devGrafanaHost))},
				{"/prod-grafana", trimPrefix("/admin/prod-grafana", newProxy(c.prodGrafanaHost))},
				{"/scope", trimPrefix("/admin/scope", newProxy(c.scopeHost))},
				{"/users", trimPrefix("/admin/users", newProxy(c.usersHost))},
				{"/kubediff", trimPrefix("/admin/kubediff", newProxy(c.kubediffHost))},
				{"/terradiff", trimPrefix("/admin/terradiff", newProxy(c.terradiffHost))},
				{"/ansiblediff", trimPrefix("/admin/ansiblediff", newProxy(c.ansiblediffHost))},
				{"/alertmanager", newProxy(c.alertmanagerHost)},
				{"/prometheus", newProxy(c.prometheusHost)},
				{"/kubedash", trimPrefix("/admin/kubedash", newProxy(c.kubedashHost))},
				{"/compare-images", trimPrefix("/admin/compare-images", newProxy(c.compareImagesHost))},
				{"/cortex/ring", trimPrefix("/admin/cortex", cortexDistributorClient)},
				{"/loki", trimPrefix("/admin/loki", newProxy(c.lokiHost))},
				{"/", http.HandlerFunc(adminRoot)},
			},
			middleware.Merge(
				// If not logged in, prompt user to log in instead of 401ing
				middleware.ErrorHandler{
					Code:    401,
					Handler: redirect("/login"),
				},
				users.AuthAdminMiddleware{
					Authenticator: c.authenticator,
					OutputHeader:  c.outputHeader,
				},
			),
		},

		// billing UI needs authentication
		path{"/billing/{jsfile}.js", trimPrefix("/billing", newProxy(c.billingUIHost))},
		// Fonts
		path{"/billing/{wofffile}.woff", trimPrefix("/billing", newProxy(c.billingUIHost))},
		path{"/billing/{ttffile}.ttf", trimPrefix("/billing", newProxy(c.billingUIHost))},
		path{"/billing/{svgfile}.svg", trimPrefix("/billing", newProxy(c.billingUIHost))},
		path{"/billing/{eotfile}.eot", trimPrefix("/billing", newProxy(c.billingUIHost))},
		path{"/billing/callback/register", trimPrefix("/billing", newProxy(c.billingUIHost))},
		prefix{
			"/billing",
			[]path{
				{"/{orgExternalID}/", trimPrefix("/billing", newProxy(c.billingUIHost))},
			},
			middleware.Merge(
				billingAuthMiddleware,
				uiHTTPlogger,
			),
		},
		// These billing api endpoints have no orgExternalID, so we can't do authorization on them.
		path{"/api/billing/accounts", trimPrefix("/api/billing", newProxy(c.billingAPIHost))},
		path{"/api/billing/payments/authTokens", trimPrefix("/api/billing", newProxy(c.billingAPIHost))},
		prefix{
			"/api/billing",
			[]path{
				{"/payments/authTokens/{orgExternalID}", newProxy(c.billingAPIHost)},
				{"/payments/{orgExternalID}", newProxy(c.billingAPIHost)},
				{"/accounts/{orgExternalID}", newProxy(c.billingAPIHost)},
				{"/usage/{orgExternalID}", newProxy(c.billingUsageHost)},
			},
			middleware.Merge(
				billingAuthMiddleware,
				middleware.PathRewrite(regexp.MustCompile("^/api/billing"), ""),
				uiHTTPlogger,
			),
		},

		// unauthenticated communication
		prefix{
			"/",
			[]path{
				{"/api/users", newProxy(c.usersHost)},
				{"/launch/k8s", newProxy(c.launchGeneratorHost)},
				{"/k8s", newProxy(c.launchGeneratorHost)},

				// rewrite /demo/* to /* and send it to demo
				{"/demo/", middleware.PathRewrite(regexp.MustCompile("/demo/(.*)"), "/$1").
					Wrap(newProxy(c.demoHost))},

				// final wildcard match to static content
				{"/", noCacheOnRoot.Wrap(newProxy(c.uiServerHost))},
			},
			uiHTTPlogger,
		},
	} {
		route.Add(r)
	}

	sameOrigin := http.Header{}
	sameOrigin.Add("X-Frame-Options", "SAMEORIGIN")
	return middleware.Merge(
		originCheckerMiddleware{expectedTarget: c.targetOrigin},
		middleware.Func(func(handler http.Handler) http.Handler {
			return nethttp.Middleware(opentracing.GlobalTracer(), handler)
		}),
		middleware.HeaderAdder{sameOrigin},
		middleware.Instrument{
			RouteMatcher: r,
			Duration:     common.RequestDuration,
		},
		middleware.Log{
			LogSuccess: c.logSuccess,
		},
		middleware.Redirect{
			Matches: []middleware.Match{
				{Host: "scope.weave.works"},
				{Scheme: "http", Host: "cloud.weave.works"},
			},
			RedirectHost:   "cloud.weave.works",
			RedirectScheme: "https",
		},
	).Wrap(r), nil
}

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
				log.Warnf("originCheckerMiddleware: Cannot parse %s header: %v", headerName, err)
				return false
			}
			return url.Host == o.expectedTarget
		}
		// If the header is missing we intentionally consider it a match
		// Some legitimate requests come without headers, e.g: Scope probe requests and non-js browser requests
		return true
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Debugf("originCheckerMiddleware: URL %s, Method %s, Origin: %q, Referer: %q, expectedTarget: %q",
			r.URL, r.Method, r.Header.Get("Origin"), r.Referer(), o.expectedTarget)
		// Verify that origin or referer headers (when present) match the expected target
		if !headerMatchesTarget("Origin", r) || !headerMatchesTarget("Referer", r) {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
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

func redirect(dest string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, dest, 302)
	})
}

type routable interface {
	Add(*mux.Router)
}

// A path routable says "map this path to this handler"
type path struct {
	path    string
	handler http.Handler
}

func (p path) Add(r *mux.Router) {
	r.Path(p.path).Name(middleware.MakeLabelValue(p.path)).Handler(p.handler)
}

// A prefix routable says "for each of these path routables, add this path prefix
// and (optionally) wrap the handlers with this middleware.
type prefix struct {
	prefix string
	routes []path
	mid    middleware.Interface
}

func (p prefix) Add(r *mux.Router) {
	if p.mid == nil {
		p.mid = middleware.Identity
	}
	for _, route := range p.routes {
		path := filepath.Join(p.prefix, route.path)
		r.
			PathPrefix(path).
			Name(middleware.MakeLabelValue(path)).
			Handler(
				p.mid.Wrap(route.handler),
			)
	}
}
