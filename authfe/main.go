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

	// Security-related flags
	flag.StringVar(&c.targetOrigin, "hostname", "", "Hostname through which this server is accessed, for same-origin checks (CSRF protection)")
	flag.BoolVar(&c.redirectHTTPS, "redirect-https", false, "Redirect all HTTP traffic to HTTPS")
	flag.IntVar(&c.hstsMaxAge, "hsts-max-age", 0, "Max Age in seconds for HSTS header - zero means no header.  Header will only be send if redirect-https is true.")
	flag.BoolVar(&c.sendCSPHeader, "send-csp-header", false, "Send \"Content-Security-Policy: default-src https:\" in all responses.")

	hostFlags := []struct {
		dest *string
		name string
	}{
		// User-visible services - keep alphabetically sorted pls.
		{&c.billingAPIHost, "billing-api"},
		{&c.billingUIHost, "billing-ui"},
		{&c.billingUsageHost, "billing-usage"},
		{&c.collectionHost, "collection"},
		{&c.configsHost, "configs"},
		{&c.controlHost, "control"},
		{&c.demoHost, "demo"},
		{&c.fluxHost, "flux"},
		{&c.launchGeneratorHost, "launch-generator"},
		{&c.pipeHost, "pipe"},
		{&c.promDistributorHost, "prom-distributor"},
		{&c.promDistributorHostGRPC, "prom-distributor-grpc"},
		{&c.promQuerierHost, "prom-querier"},
		{&c.promQuerierHostGRPC, "prom-querier-grpc"},
		{&c.queryHost, "query"},
		{&c.uiMetricsHost, "ui-metrics"},
		{&c.uiServerHost, "ui-server"},

		// Admin services - keep alphabetically sorted pls.
		{&c.alertmanagerHost, "alertmanager"},
		{&c.ansiblediffHost, "ansiblediff"},
		{&c.compareImagesHost, "compare-images"},
		{&c.devGrafanaHost, "dev-grafana"},
		{&c.grafanaHost, "grafana"},
		{&c.kubedashHost, "kubedash"},
		{&c.kubediffHost, "kubediff"},
		{&c.lokiHost, "loki"},
		{&c.prodGrafanaHost, "prod-grafana"},
		{&c.prometheusHost, "prometheus"},
		{&c.scopeHost, "scope"},
		{&c.terradiffHost, "terradiff"},
		{&c.usersHost, "users"},
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

	// We run up to 3 HTTP servers on 2 ports, listening in various ways:
	//
	// - one on port 8080 of this pod, for metrics and traces
	// - one or two on port 80, routed based on the destination port on the ELB -
	//   (discovered using proxy protocol):
	//   - port 443 serving "real traffic"
	//   - on all other ports redirecting to SSL
	//
	// If the HTTP redirect is disabled, then the "real traffic" server will serve
	// traffic for all ports on the ELB.

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
	var proxyListener net.Listener = &proxyproto.Listener{
		Listener:           listener,
		ProxyHeaderTimeout: proxyTimeout,
	}

	if c.redirectHTTPS {
		// We use a custom listener router to ensure only connections on port 443 get
		// through to the real router - everything else gets redirected.
		proxyListenerRouter := newProxyListenerRouter(proxyListener)
		proxyListener = proxyListenerRouter.listenerForPort(443)
		redirectServer := &http.Server{
			Handler: c.commonMiddleWare(nil).Wrap(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					url := r.URL
					url.Host = r.Host
					url.Scheme = "https"
					http.Redirect(w, r, url.String(), http.StatusMovedPermanently)
				}),
			),
		}
		go redirectServer.Serve(proxyListenerRouter)
	}

	// block until stop signal is received, then wait stopTimeout for remaining conns
	if err := server.Serve(proxyListener); err != nil {
		log.Fatal(err)
	}
	log.Info("Gracefully shut down")
}
