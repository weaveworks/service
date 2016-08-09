package main

import (
	"flag"
	"net/http"
	"regexp"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/weaveworks/scope/common/xfer"
	"github.com/weaveworks/service/common/logging"
	users "github.com/weaveworks/service/users/client"
)

var (
	requestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "scope",
		Name:      "request_duration_seconds",
		Help:      "Time (in seconds) spent serving HTTP requests.",
		Buckets:   prometheus.DefBuckets,
	}, []string{"method", "route", "status_code", "ws"})
	wsConnections = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "scope",
		Name:      "websocket_connection_count",
		Help:      "Number of currently active websocket connections.",
	})
	wsRequestCount = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "scope",
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

func newProbeRequestLogger(orgIDHeader string) logging.HTTPEventExtractor {
	return func(r *http.Request) (logging.Event, bool) {
		event := logging.Event{
			ID:             r.URL.Path,
			Product:        "scope-probe",
			Version:        r.Header.Get(xfer.ScopeProbeVersionHeader),
			UserAgent:      r.UserAgent(),
			ClientID:       r.Header.Get(xfer.ScopeProbeIDHeader),
			OrganizationID: r.Header.Get(orgIDHeader),
		}
		return event, true
	}
}

func newUIRequestLogger(orgIDHeader string) logging.HTTPEventExtractor {
	return func(r *http.Request) (logging.Event, bool) {
		event := logging.Event{
			ID:             r.URL.Path,
			Product:        "scope-ui",
			UserAgent:      r.UserAgent(),
			OrganizationID: r.Header.Get(orgIDHeader),
			// TODO: fill in after implementing user support in organizations
			// UserID: "" ,
		}
		return event, true
	}
}

func main() {
	var (
		listen              string
		logLevel            string
		authType            string
		authURL             string
		authCacheSize       int
		authCacheExpiration time.Duration
		fluentHost          string

		c Config
	)
	flag.StringVar(&listen, "listen", ":80", "HTTP server listen address")
	flag.StringVar(&logLevel, "log.level", "info", "Logging level to use: debug | info | warn | error")
	flag.StringVar(&authType, "authenticator", "web", "What authenticator to use: web | mock")
	flag.StringVar(&authURL, "authenticator.url", "http://users:80", "Where to find web the authenticator service")
	flag.IntVar(&authCacheSize, "auth.cache.size", 0, "How many entries to cache in the auth client.")
	flag.DurationVar(&authCacheExpiration, "auth.cache.expiration", 30*time.Second, "How long to keep entries in the auth client.")
	flag.StringVar(&fluentHost, "fluent", "", "Hostname & port for fluent")
	flag.StringVar(&c.outputHeader, "output.header", "X-Scope-OrgID", "Name of header containing org id on forwarded requests")
	flag.StringVar(&c.collectionHost, "collection", "collection.default.svc.cluster.local:80", "Hostname & port for collection service")
	flag.StringVar(&c.queryHost, "query", "query.default.svc.cluster.local:80", "Hostname & port for query service")
	flag.StringVar(&c.controlHost, "control", "control.default.svc.cluster.local:80", "Hostname & port for control service")
	flag.StringVar(&c.pipeHost, "pipe", "pipe.default.svc.cluster.local:80", "Hostname & port for pipe service")
	flag.StringVar(&c.deployHost, "deploy", "api.deploy.svc.cluster.local:80", "Hostname & port for deploy service")
	flag.StringVar(&c.promHost, "prom", "distributor.frankenstein.svc.cluster.local:80", "Hostname & port for prom service")

	// For Admin routers
	flag.StringVar(&c.grafanaHost, "grafana", "grafana.monitoring.svc.cluster.local:80", "Hostname & port for gafana")
	flag.StringVar(&c.scopeHost, "scope", "scope.kube-system.svc.cluster.local:80", "Hostname & port for scope")
	flag.StringVar(&c.usersHost, "users", "users.default.svc.cluster.local", "Hostname & port for users")
	flag.StringVar(&c.kubediffHost, "kubediff", "kubediff.monitoring.svc.cluster.local", "Hostname & port for kubediff")
	flag.StringVar(&c.alertmanagerHost, "alertmanager", "alertmanager.monitoring.svc.cluster.local", "Hostname & port for alertmanager")
	flag.StringVar(&c.prometheusHost, "prometheus", "prometheus.monitoring.svc.cluster.local", "Hostname & port for prometheus")
	flag.StringVar(&c.kubedashHost, "kubedash", "kubernetes-dashboard.kube-system.svc.cluster.local", "Hostname & port for kubedash")
	flag.Parse()

	if err := logging.Setup(logLevel); err != nil {
		log.Fatalf("Error configuring logging: %v", err)
		return
	}

	authOptions := users.AuthenticatorOptions{}
	if authCacheSize > 0 {
		authOptions.CredCacheEnabled = true
		authOptions.OrgCredCacheSize = authCacheSize
		authOptions.ProbeCredCacheSize = authCacheSize
		authOptions.OrgCredCacheExpiration = authCacheExpiration
		authOptions.ProbeCredCacheExpiration = authCacheExpiration
	}
	c.authenticator = users.MakeAuthenticator(authType, authURL, authOptions)

	if fluentHost != "" {
		var err error
		c.eventLogger, err = logging.NewEventLogger(fluentHost)
		if err != nil {
			log.Fatalf("Error setting up event logging: %v", err)
		}
		defer c.eventLogger.Close()
	}

	r, err := routes(c)
	if err != nil {
		log.Fatal(err)
	}
	log.Infof("Listening on %s", listen)
	log.Fatal(http.ListenAndServe(listen, r))
}
