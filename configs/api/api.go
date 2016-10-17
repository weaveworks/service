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

// getConfig returns the requested configuration.
func (a *API) getUserConfig(w http.ResponseWriter, r *http.Request) {
	actualUserID := r.Header.Get(a.UserIDHeader)
	if actualUserID == "" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	vars := mux.Vars(r)
	requestedUserID := vars["userID"]
	if requestedUserID != actualUserID {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	w.WriteHeader(http.StatusNotFound)
}
