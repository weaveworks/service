package api

import (
	"net/http"

	"github.com/gorilla/mux"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/weaveworks/scope/common/middleware"
)

var (
	requestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "config", // XXX: Should this be 'scope'?
		Name:      "request_duration_seconds",
		Help:      "Time (in seconds) spent serving HTTP requests.",
		Buckets:   prometheus.DefBuckets,
	}, []string{"method", "route", "status_code", "ws"})
)

func init() {
	prometheus.MustRegister(requestDuration)
}

// API implements the config api.
type API struct {
	http.Handler
}

// New creates a new API
func New() *API {
	a := &API{}
	a.Handler = a.routes()
	return a
}

func (a *API) routes() http.Handler {
	r := mux.NewRouter()
	for _, route := range []struct {
		name, method, path string
		handler            http.HandlerFunc
	}{} {
		r.Handle(route.path, route.handler).Methods(route.method).Name(route.name)
	}
	return middleware.Merge(
		middleware.Logging,
		middleware.Instrument{
			RouteMatcher: r,
			Duration:     requestDuration,
		},
	).Wrap(r)
}
