package main

import (
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"path/filepath"
	"regexp"

	log "github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/weaveworks/common/middleware"
	"github.com/weaveworks/scope/common/xfer"

	users "github.com/weaveworks/service/users/client"
)

const maxAnalyticsPayloadSize = 16 * 1024 // bytes

// Config is all the config we need to build the routes
type Config struct {
	authenticator       users.Authenticator
	eventLogger         *EventLogger
	outputHeader        string
	collectionHost      string
	queryHost           string
	controlHost         string
	pipeHost            string
	deployHost          string
	fluxHost            string
	grafanaHost         string
	devGrafanaHost      string
	prodGrafanaHost     string
	scopeHost           string
	usersHost           string
	kubediffHost        string
	terradiffHost       string
	ansiblediffHost     string
	alertmanagerHost    string
	prometheusHost      string
	kubedashHost        string
	promHost            string
	compareImagesHost   string
	uiServerHost        string
	billingUIHost       string
	billingAPIHost      string
	billingUsageHost    string
	demoHost            string
	launchGeneratorHost string
	uiMetricsHost       string
	logSuccess          bool
	apiInfo             string
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

	r := newRouter()
	for _, route := range []routable{
		path{"/metrics", prometheus.Handler()},

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
				{"/api/deploy", newProxy(c.deployHost)},
				{"/api/config", newProxy(c.deployHost)},
				{"/api/flux", trimPrefix("/api/flux", newProxy(c.fluxHost))},
				{"/api/prom", newProxy(c.promHost)},
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
				{"/deploy", newProxy(c.deployHost)},
				{"/config", newProxy(c.deployHost)},
				{"/flux", trimPrefix("/api/flux", newProxy(c.fluxHost))},
				{"/prom", newProxy(c.promHost)},
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
				{"/cortex/ring", trimPrefix("/admin/cortex", newProxy(c.promHost))},
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

	return middleware.Merge(
		middleware.Instrument{
			RouteMatcher: r,
			Duration:     requestDuration,
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
