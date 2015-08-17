package instrument

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
)

// Middleware records the latency of HTTP requests that flow through it.
// RouteMatcher is implemented by a mux.Router, provided you configure each
// path with a name. (See NamedPath and NamedPathPrefix.) The Prometheus
// summary MUST be configured with the labels method, route, and status code,
// in that order.
func Middleware(m RouteMatcher, duration *prometheus.SummaryVec) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			begin := time.Now()
			i := interceptor{ResponseWriter: w}
			next.ServeHTTP(i, r)
			var (
				method = r.Method
				route  = getRouteName(m, r)
				status = strconv.Itoa(i.statusCode)
				took   = time.Since(begin)
			)
			logrus.Infof("%s: %s %s (%s) %s", r.URL.Path, method, route, status, took)
			duration.WithLabelValues(method, route, status).Observe(float64(took.Nanoseconds()))
		})
	}
}

// RouteMatcher is implemented by mux.Router.
type RouteMatcher interface {
	Match(*http.Request, *mux.RouteMatch) bool
}

type interceptor struct {
	http.ResponseWriter
	statusCode int
}

func (i interceptor) WriteHeader(code int) {
	i.statusCode = code
	i.ResponseWriter.WriteHeader(code)
}

func getRouteName(m RouteMatcher, r *http.Request) string {
	var routeMatch mux.RouteMatch
	if !m.Match(r, &routeMatch) {
		return "unmatched_path"
	}
	name := routeMatch.Route.GetName()
	if name == "" {
		return "unnamed_path"
	}
	return name
}

var dedupe = strings.NewReplacer("__", "_")
