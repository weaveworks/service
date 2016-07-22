package main

import (
	"fmt"
	"net/http"
	"path/filepath"
	"regexp"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/weaveworks/scope/common/middleware"
	"github.com/weaveworks/service/common/logging"
	users "github.com/weaveworks/service/users/client"
)

// Config is all the config we need to build the routes
type Config struct {
	authenticator    users.Authenticator
	eventLogger      *logging.EventLogger
	outputHeader     string
	collectionHost   string
	queryHost        string
	controlHost      string
	pipeHost         string
	deployHost       string
	fluentHost       string
	grafanaHost      string
	scopeHost        string
	usersHost        string
	kubediffHost     string
	alertmanagerHost string
	prometheusHost   string
}

func routes(c Config) (http.Handler, error) {
	probeHTTPlogger := middleware.Identity
	uiHTTPlogger := middleware.Identity
	if c.eventLogger != nil {
		probeHTTPlogger = logging.HTTPEventLogger{
			Extractor: newProbeRequestLogger(c.outputHeader),
			Logger:    c.eventLogger,
		}
		uiHTTPlogger = logging.HTTPEventLogger{
			Extractor: newUIRequestLogger(c.outputHeader),
			Logger:    c.eventLogger,
		}
	}

	r := newRouter()
	for _, route := range []routable{
		path{"/loadgen", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			fmt.Fprintf(w, "OK")
		})},

		path{"/metrics", prometheus.Handler()},

		// For all ui <-> app communication, authenticated using cookie credentials
		prefix{
			"/api/app/{orgName}",
			map[string]http.Handler{
				"/api/report":   newProxy(c.queryHost),
				"/api/topology": newProxy(c.queryHost),
				"/api/control":  newProxy(c.controlHost),
				"/api/pipe":     newProxy(c.pipeHost),
				"/api/deploy":   newProxy(c.deployHost),
				"/api/config":   newProxy(c.deployHost),
				"/":             newProxy(c.queryHost), // catch all forward to query service, for /api and static html
			},
			middleware.Merge(
				users.AuthOrgMiddleware{
					Authenticator: c.authenticator,
					OrgName: func(r *http.Request) (string, bool) {
						v, ok := mux.Vars(r)["orgName"]
						return v, ok
					},
					OutputHeader: c.outputHeader,
				},
				middleware.PathRewrite(regexp.MustCompile("^/api/app/[^/]+"), ""),
				uiHTTPlogger,
			),
		},

		// For all probe <-> app communication, authenticated using header credentials
		prefix{
			"/api",
			map[string]http.Handler{
				"/report":  newProxy(c.collectionHost),
				"/control": newProxy(c.controlHost),
				"/pipe":    newProxy(c.pipeHost),
				"/deploy":  newProxy(c.deployHost),
				"/config":  newProxy(c.deployHost),
			},
			middleware.Merge(
				users.AuthProbeMiddleware{
					Authenticator: c.authenticator,
					OutputHeader:  c.outputHeader,
				},
				probeHTTPlogger,
			),
		},

		// For all admin functionality, authenticated using header credentials
		prefix{
			"/admin",
			map[string]http.Handler{
				"/grafana":      trim("^/admin/grafana", newProxy(c.grafanaHost)),
				"/scope":        trim("^/admin/scope", newProxy(c.scopeHost)),
				"/users":        trim("^/admin/users", newProxy(c.usersHost)),
				"/kubediff":     trim("^/admin/kubediff", newProxy(c.kubediffHost)),
				"/alertmanager": newProxy(c.alertmanagerHost),
				"/prometheus":   newProxy(c.prometheusHost),
				"/":             http.HandlerFunc(adminRoot),
			},
			users.AuthAdminMiddleware{
				Authenticator: c.authenticator,
				OutputHeader:  c.outputHeader,
			},
		},
	} {
		route.Add(r)
	}

	return middleware.Merge(
		middleware.Instrument{
			RouteMatcher: r,
			Duration:     requestDuration,
		},
		middleware.Logging,
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

func trim(regex string, handler http.Handler) http.Handler {
	return middleware.PathRewrite(regexp.MustCompile(regex), "").Wrap(handler)
}

type routable interface {
	Add(*mux.Router)
}

type path struct {
	path    string
	handler http.Handler
}

func (p path) Add(r *mux.Router) {
	r.Path(p.path).Name(middleware.MakeLabelValue(p.path)).Handler(p.handler)
}

type prefix struct {
	prefix   string
	handlers map[string]http.Handler
	mid      middleware.Interface
}

func (p prefix) Add(r *mux.Router) {
	if p.mid == nil {
		p.mid = middleware.Identity
	}
	for path, handler := range p.handlers {
		path = filepath.Join(p.prefix, path)
		r.
			PathPrefix(path).
			Name(middleware.MakeLabelValue(path)).
			Handler(
				p.mid.Wrap(handler),
			)
	}
}
