package instrument

import (
	"bufio"
	"fmt"
	"net"
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
			i := &interceptor{ResponseWriter: w, statusCode: http.StatusOK}
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

// interceptor implements WriteHeader to intercept status codes. WriteHeader
// may not be called on success, so initialize statusCode with the status you
// want to report on success, i.e. http.StatusOK.
//
// interceptor also implements net.Hijacker, to let the downstream Handler
// hijack the connection. This is needed by the app-mapper's proxy.
type interceptor struct {
	http.ResponseWriter
	statusCode int
}

func (i *interceptor) WriteHeader(code int) {
	i.statusCode = code
	i.ResponseWriter.WriteHeader(code)
}

func (i *interceptor) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj, ok := i.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("interceptor: can't cast parent ResponseWriter to Hijacker")
	}
	return hj.Hijack()
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
