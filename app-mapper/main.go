package main

import (
	"net/http"
	"strconv"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

func main() {
	flags := parseFlags()
	setupLogging(flags.logLevel)

	authenticator := flags.getAuthenticator()
	appProvisioner := flags.getAppProvisioner()
	logrus.Info("Fetching application")
	err := appProvisioner.fetchApp()
	if err != nil {
		logrus.Error("Couldn't fetch application: ", err)
	}
	db, err := sqlx.Open("postgres", flags.dbURI)
	if err != nil {
		logrus.Fatal("Cannot initialize database client: ", err)
	}
	orgMapper := flags.getOrganizationMapper(db, appProvisioner)
	probeStorage := newProbeDBStorage(db)

	router := mux.NewRouter()
	newProxy(authenticator, orgMapper, probeStorage).registerHandlers(router)
	newProbeObserver(authenticator, probeStorage).registerHandlers(router)
	http.Handle("/metrics", makePrometheusHandler())
	http.Handle("/", instrument(router, router))
	logrus.Infof("Listening on %s", flags.listen)
	logrus.Fatal(http.ListenAndServe(flags.listen, nil))
}

func instrument(m routeMatcher, next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		begin := time.Now()
		logrus.Infof("%s %s %s", r.RemoteAddr, r.Method, r.URL)
		i := interceptor{ResponseWriter: w}

		next.ServeHTTP(&i, r)

		var (
			method = r.Method
			route  = normalizePath(m, r)
			status = strconv.Itoa(i.statusCode)
			took   = time.Since(begin)
		)
		logrus.Debugf("%s: %s %s (%s) %s", r.URL.Path, method, route, status, took)
		requestDuration.WithLabelValues(method, route, status).Observe(float64(took.Nanoseconds()))
	}
}

type routeMatcher interface {
	Match(*http.Request, *mux.RouteMatch) bool
}

type interceptor struct {
	http.ResponseWriter
	statusCode int
}

func (i *interceptor) WriteHeader(code int) {
	i.statusCode = code
	i.ResponseWriter.WriteHeader(code)
}

func normalizePath(m routeMatcher, r *http.Request) string {
	var match mux.RouteMatch
	if !m.Match(r, &match) {
		logrus.Warnf("couldn't normalize path: %s", r.URL.Path)
		return "unmatched_path"
	}
	name := match.Route.GetName()
	if name == "" {
		logrus.Warnf("path isn't named: %s", r.URL.Path)
		return "unnamed_path"
	}
	return name
}
