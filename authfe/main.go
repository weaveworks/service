package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/armon/go-proxyproto"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/tylerb/graceful"

	"github.com/weaveworks/common/logging"
	"github.com/weaveworks/service/common"
	users "github.com/weaveworks/service/users/client"
)

const (
	sessionCookieKey   = "_weaveclientid"
	userIDHeader       = "X-Scope-UserID"
	featureFlagsHeader = "X-FeatureFlags"
	proxyTimeout       = 30 * time.Second
)

var (
	wsConnections = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: common.PrometheusNamespace,
		Name:      "websocket_connection_count",
		Help:      "Number of currently active websocket connections.",
	})
	wsRequestCount = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: common.PrometheusNamespace,
		Name:      "websocket_request_count",
		Help:      "Total number of websocket requests received.",
	})
	orgPrefix = regexp.MustCompile("^/api/app/[^/]+")
)

func init() {
	prometheus.MustRegister(wsConnections)
	prometheus.MustRegister(wsRequestCount)
}

func main() {
	var (
		listen, privateListen string
		stopTimeout           time.Duration
		logLevel              string
		authType              string
		authURL               string
		authCacheSize         int
		authCacheExpiration   time.Duration
		fluentHost            string

		c Config
	)
	flag.StringVar(&listen, "listen", ":80", "HTTP server listen address")
	flag.StringVar(&privateListen, "private-listen", ":8080", "HTTP server listen address (private endpoints)")
	flag.DurationVar(&stopTimeout, "stop.timeout", 5*time.Second, "How long to wait for remaining requests to finish during shutdown")
	flag.StringVar(&logLevel, "log.level", "info", "Logging level to use: debug | info | warn | error")
	flag.BoolVar(&c.logSuccess, "log.success", false, "Log successful requests.")
	flag.StringVar(&authType, "authenticator", "web", "What authenticator to use: web | mock")
	flag.StringVar(&authURL, "authenticator.url", "http://users:80", "Where to find web the authenticator service")
	flag.IntVar(&authCacheSize, "auth.cache.size", 0, "How many entries to cache in the auth client.")
	flag.DurationVar(&authCacheExpiration, "auth.cache.expiration", 30*time.Second, "How long to keep entries in the auth client.")
	flag.StringVar(&fluentHost, "fluent", "", "Hostname & port for fluent")
	flag.StringVar(&c.outputHeader, "output.header", "X-Scope-OrgID", "Name of header containing org id on forwarded requests")
	flag.StringVar(&c.apiInfo, "api.info", "scopeservice:0.1", "Version info for the api to serve, in format ID:VERSION")
	flag.StringVar(&c.targetOrigin, "hostname", "", "Hostname through which this server is accessed, for same-origin checks (CSRF protection)")

	hostFlags := []struct {
		dest *string
		name string
	}{
		{&c.deployHost, "deploy"},
		{&c.fluxHost, "flux"},
		{&c.promHost, "prom"}, // for backwards compatibility
		{&c.promDistributorHost, "prom-distributor"},
		{&c.promQuerierHost, "prom-querier"},
		{&c.promDistributorHostGRPC, "prom-distributor-grpc"},
		{&c.promQuerierHostGRPC, "prom-querier-grpc"},
		{&c.configsHost, "configs"},
		{&c.collectionHost, "collection"},
		{&c.queryHost, "query"},
		{&c.controlHost, "control"},
		{&c.pipeHost, "pipe"},
		// For Admin routers
		{&c.grafanaHost, "grafana"},
		{&c.devGrafanaHost, "dev-grafana"},
		{&c.prodGrafanaHost, "prod-grafana"},
		{&c.scopeHost, "scope"},
		{&c.usersHost, "users"},
		{&c.kubediffHost, "kubediff"},
		{&c.terradiffHost, "terradiff"},
		{&c.ansiblediffHost, "ansiblediff"},
		{&c.alertmanagerHost, "alertmanager"},
		{&c.prometheusHost, "prometheus"},
		{&c.kubedashHost, "kubedash"},
		{&c.compareImagesHost, "compare-images"},
		{&c.uiServerHost, "ui-server"},
		{&c.billingUIHost, "billing-ui"},
		{&c.billingAPIHost, "billing-api"},
		{&c.billingUsageHost, "billing-usage"},
		{&c.demoHost, "demo"},
		{&c.launchGeneratorHost, "launch-generator"},
		{&c.uiMetricsHost, "ui-metrics"},
	}

	for _, hostFlag := range hostFlags {
		flag.StringVar(hostFlag.dest, hostFlag.name, "", fmt.Sprintf("Hostname & port for %s service", hostFlag.name))
	}

	flag.Parse()

	if err := logging.Setup(logLevel); err != nil {
		log.Fatalf("Error configuring logging: %v", err)
		return
	}

	for _, hostFlag := range hostFlags {
		if *hostFlag.dest == "" {
			log.Warningf("Host for %s not given; will not be proxied", hostFlag.name)
		}
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
		c.eventLogger, err = NewEventLogger(fluentHost)
		if err != nil {
			log.Fatalf("Error setting up event logging: %v", err)
		}
		defer c.eventLogger.Close()
	}

	log.Infof("Listening on %s for private endpoints", privateListen)
	privListener, err := net.Listen("tcp", privateListen)
	if err != nil {
		log.Fatal(err)
	}

	privRouter, err := privateRoutes()
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		if err := http.Serve(privListener, privRouter); err != nil {
			log.Fatal(err)
		}
	}()

	log.Infof("Listening on %s", listen)
	listener, err := net.Listen("tcp", listen)
	if err != nil {
		log.Fatal(err)
	}

	r, err := routes(c)
	if err != nil {
		log.Fatal(err)
	}

	server := &graceful.Server{
		Timeout: stopTimeout,
		Server: &http.Server{
			Handler: r,
		},
	}
	proxyListener := &proxyproto.Listener{
		Listener:           listener,
		ProxyHeaderTimeout: proxyTimeout,
	}

	// block until stop signal is received, then wait stopTimeout for remaining conns
	if err := server.Serve(proxyListener); err != nil {
		log.Fatal(err)
	}
	log.Info("Gracefully shut down")
}
