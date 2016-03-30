package main

import (
	"flag"
	"fmt"
	"net/http"
	"regexp"

	log "github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/weaveworks/scope/common/middleware"

	"github.com/weaveworks/service/common/logging"
	users "github.com/weaveworks/service/users/client"
)

var (
	requestDuration = prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Namespace: "scope",
		Subsystem: "authfe",
		Name:      "request_duration_nanoseconds",
		Help:      "Time spent serving HTTP requests.",
	}, []string{"method", "route", "status_code"})
	wsConnections = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "scope",
		Subsystem: "authfe",
		Name:      "websocket_connection_count",
		Help:      "Number of currently active websocket connections.",
	})
	wsRequestCount = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "scope",
		Subsystem: "authfe",
		Name:      "websocket_request_count",
		Help:      "Total number of websocket requests received.",
	})
	orgPrefix = regexp.MustCompile("^/api/app/[^/]+")
)

func init() {
	prometheus.MustRegister(requestDuration)
	prometheus.MustRegister(wsConnections)
	prometheus.MustRegister(wsRequestCount)
}

func trimOrgPrfix(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.RequestURI = orgPrefix.ReplaceAllLiteralString(r.RequestURI, "")
		next.ServeHTTP(w, r)
	})
}

func main() {
	var (
		listen            string
		logLevel          string
		authenticatorType string
		authenticatorURL  string
		outputHeader      string
		collectionHost    string
		queryHost         string
		controlHost       string
		pipeHost          string
	)
	flag.StringVar(&listen, "listen", ":80", "HTTP server listen address")
	flag.StringVar(&logLevel, "log.level", "info", "Logging level to use: debug | info | warn | error")
	flag.StringVar(&authenticatorType, "authenticator", "web", "What authenticator to use: web | mock")
	flag.StringVar(&authenticatorURL, "authenticator.url", "http://users:80", "Where to find web the authenticator service")
	flag.StringVar(&outputHeader, "output.header", "X-Scope-OrgID", "Name of header containing org id on forwarded requests")
	flag.StringVar(&collectionHost, "collection", "collection.default.svc.cluster.local:80", "")
	flag.StringVar(&queryHost, "query", "query.default.svc.cluster.local:80", "")
	flag.StringVar(&controlHost, "control", "control.default.svc.cluster.local:80", "")
	flag.StringVar(&pipeHost, "pipe", "pipe.default.svc.cluster.local:80", "")
	flag.Parse()

	if err := logging.Setup(logLevel); err != nil {
		log.Fatalf("Error configuring logging: %v", err)
		return
	}

	// these are the places we can forward requests to
	collectionFwd := newProxy(collectionHost)
	queryFwd := newProxy(queryHost)
	contolFwd := newProxy(controlHost)
	pipeFwd := newProxy(pipeHost)

	// orgRouter is for all ui <-> app communication, authenticated using cookie credentials
	orgRouter := mux.NewRouter().StrictSlash(false)
	orgRouter.PathPrefix("/api").Name("api_app").Handler(queryFwd)
	orgRouter.PathPrefix("/api/report").Name("api_app_report").Handler(queryFwd)
	orgRouter.PathPrefix("/api/topology").Name("api_app_topology").Handler(queryFwd)
	orgRouter.PathPrefix("/api/control").Name("api_app_control").Handler(contolFwd)
	orgRouter.PathPrefix("/api/pipe").Name("api_app_pipe").Handler(pipeFwd)

	// probeRouter is for all probe <-> app communication, authenticated using header credentials
	probeRouter := mux.NewRouter().StrictSlash(false)
	probeRouter.PathPrefix("/api/report").Name("api_probe_report").Handler(collectionFwd)
	probeRouter.PathPrefix("/api/control").Name("api_probe_control").Handler(contolFwd)
	probeRouter.PathPrefix("/api/pipe").Name("api_probe_pipe").Handler(pipeFwd)

	// authentication is done by middleware
	authenticator := users.MakeAuthenticator(authenticatorType, authenticatorURL)
	orgAuthMiddleware := users.AuthOrgMiddleware{
		Authenticator: authenticator,
		OrgName: func(r *http.Request) (string, bool) {
			v, ok := mux.Vars(r)["orgName"]
			return v, ok
		},
		OutputHeader: outputHeader,
	}
	probeAuthMiddleware := users.AuthProbeMiddleware{
		Authenticator: authenticator,
		OutputHeader:  outputHeader,
	}
	orgInstrumentation := middleware.Instrument{
		RouteMatcher: orgRouter,
		Duration:     requestDuration,
	}
	probeInstrumentation := middleware.Instrument{
		RouteMatcher: probeRouter,
		Duration:     requestDuration,
	}

	// bring it all together in the root router
	rootRouter := mux.NewRouter().StrictSlash(false)
	rootRouter.Path("/loadgen").Name("loadgen").Methods("GET").HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, "OK")
	})
	rootRouter.Path("/metrics").Handler(prometheus.Handler())
	rootRouter.PathPrefix("/api/app/{orgName}").Handler(
		middleware.Merge(
			orgInstrumentation,
			orgAuthMiddleware,
			middleware.Func(trimOrgPrfix),
		).Wrap(orgRouter),
	)
	rootRouter.PathPrefix("/api").Handler(
		middleware.Merge(
			probeInstrumentation,
			probeAuthMiddleware,
		).Wrap(probeRouter))
	log.Infof("Listening on %s", listen)
	log.Fatal(http.ListenAndServe(listen, middleware.Logging.Wrap(rootRouter)))
}
