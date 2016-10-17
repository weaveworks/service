package api

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/weaveworks/scope/common/middleware"
)

const (
	defaultUserIDHeader = "X-Scope-UserID"
	defaultOrgIDHeader  = "X-Scope-OrgID"
)

var (
	requestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "configs", // XXX: Should this be 'scope'?
		Name:      "request_duration_seconds",
		Help:      "Time (in seconds) spent serving HTTP requests.",
		Buckets:   prometheus.DefBuckets,
	}, []string{"method", "route", "status_code", "ws"})
)

func init() {
	prometheus.MustRegister(requestDuration)
}

// API implements the configs api.
type API struct {
	http.Handler
	Config
}

// Config describes the configuration for the configs API.
type Config struct {
	LogSuccess   bool
	UserIDHeader string
	OrgIDHeader  string
}

// DefaultConfig returns the default configuration.
func DefaultConfig() Config {
	return Config{
		LogSuccess:   false,
		UserIDHeader: defaultUserIDHeader,
		OrgIDHeader:  defaultOrgIDHeader,
	}
}

// UserID is how users are identified.
type UserID string

// OrgID is how organizations are identified.
type OrgID string

// Subsystem is the name of a subsystem that has configuration. e.g. "deploy",
// "prism".
type Subsystem string

// New creates a new API
func New(config Config) *API {
	a := &API{Config: config}
	a.Handler = a.routes()
	return a
}

func (a *API) admin(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/html")
	fmt.Fprintf(w, `
<!doctype html>
<html>
	<head><title>configs :: configuration service</title></head>
	<body>
		<h1>configs :: configuration service</h1>
	</body>
</html>
`)
}

func (a *API) routes() http.Handler {
	r := mux.NewRouter()
	for _, route := range []struct {
		name, method, path string
		handler            http.HandlerFunc
	}{
		{"root", "GET", "/", a.admin},
		{"get_user_config", "GET", "/api/configs/user/{userID}/{subsystem}", a.getUserConfig},
		{"get_org_config", "GET", "/api/configs/org/{orgID}/{subsystem}", a.getOrgConfig},
	} {
		r.Handle(route.path, route.handler).Methods(route.method).Name(route.name)
	}
	return middleware.Merge(
		middleware.Log{
			LogSuccess: a.LogSuccess,
		},
		middleware.Instrument{
			RouteMatcher: r,
			Duration:     requestDuration,
		},
	).Wrap(r)
}

// authorize checks whether the given header provides access to entity.
func authorize(r *http.Request, header, entityID string) (string, int) {
	token := r.Header.Get(header)
	if token == "" {
		return "", http.StatusUnauthorized
	}
	vars := mux.Vars(r)
	entity := vars[entityID]
	if token != entity {
		return "", http.StatusForbidden
	}
	return entity, 0
}

// getUserConfig returns the requested configuration.
func (a *API) getUserConfig(w http.ResponseWriter, r *http.Request) {
	_, code := authorize(r, a.UserIDHeader, "userID")
	if code != 0 {
		w.WriteHeader(code)
		return
	}
	w.WriteHeader(http.StatusNotFound)
}

// getOrgConfig returns the request configuration.
func (a *API) getOrgConfig(w http.ResponseWriter, r *http.Request) {
	_, code := authorize(r, a.OrgIDHeader, "orgID")
	if code != 0 {
		w.WriteHeader(code)
		return
	}
	w.WriteHeader(http.StatusNotFound)
}
