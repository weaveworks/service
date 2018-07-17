package main

import (
	"flag"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/armon/go-proxyproto"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"github.com/tylerb/graceful"

	"github.com/weaveworks/common/logging"
	"github.com/weaveworks/common/tracing"
	"github.com/weaveworks/common/user"
	"github.com/weaveworks/service/common"
	users "github.com/weaveworks/service/users/client"
)

const (
	sessionCookieKey   = "_weaveclientid"
	userIDHeader       = user.UserIDHeaderName
	featureFlagsHeader = "X-FeatureFlags"
	proxyTimeout       = 30 * time.Second

	// The next two strings are copied from the Scope repo in order to avoid a dependency,
	// so that the transition can be managed here if Scope ever changes its headers.
	probeIDHeader      = "X-Scope-Probe-ID" // set to a random string on probe startup.
	probeVersionHeader = "X-Scope-Probe-Version"
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
	eventsDiscardedCount = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: common.PrometheusNamespace,
		Name:      "bi_events_discarded_count",
		Help:      "Total number BI events discarded.",
	})
	orgPrefix = regexp.MustCompile("^/api/app/[^/]+")
)

func init() {
	prometheus.MustRegister(wsConnections)
	prometheus.MustRegister(wsRequestCount)
	prometheus.MustRegister(eventsDiscardedCount)
	prometheus.MustRegister(common.RequestDuration)
}

func main() {
	traceCloser := tracing.NewFromEnv("authfe")
	defer traceCloser.Close()

	var (
		cfg Config
	)
	cfg.ReadEnvVars()
	cfg.RegisterFlags(flag.CommandLine)
	flag.Parse()

	if err := logging.Setup(cfg.logLevel); err != nil {
		log.Fatalf("Error configuring logging: %v", err)
		return
	}

	// Guard against accidentally disabling CSRF protection
	for _, suffix := range cfg.allowedOriginSuffixes {
		if suffix == "" {
			log.Fatalf("Empty origin suffix not permitted")
		}
	}

	// Initialize all the proxies
	for name, proxyCfg := range cfg.proxies() {
		if proxyCfg.hostAndPort == "" && proxyCfg.grpcHost == "" {
			log.Warningf("Host for %s not given; will not be proxied", name)
		}
		handler, err := newProxy(*proxyCfg)
		if err != nil {
			log.Fatal(err)
		}
		proxyCfg.Handler = handler
	}

	authOptions := users.CachingClientConfig{}
	if cfg.authCacheSize > 0 {
		authOptions.CacheEnabled = true
		authOptions.OrgCredCacheSize = cfg.authCacheSize
		authOptions.ProbeCredCacheSize = cfg.authCacheSize
		authOptions.UserCacheSize = cfg.authCacheSize
		authOptions.OrgCredCacheExpiration = cfg.authCacheExpiration
		authOptions.ProbeCredCacheExpiration = cfg.authCacheExpiration
		authOptions.UserCacheExpiration = cfg.authCacheExpiration
	}
	authenticator, err := users.New(cfg.authType, cfg.authURL, authOptions)
	if err != nil {
		log.Fatalf("Error making users client: %v", err)
		return
	}
	ghIntegration := &users.TokenRequester{
		URL:          cfg.authHTTPURL,
		UserIDHeader: userIDHeader,
	}

	var eventLogger *EventLogger
	if cfg.fluentHost != "" {
		var err error
		eventLogger, err = NewEventLogger(cfg.fluentHost)
		if err != nil {
			log.Fatalf("Error setting up event logging: %v", err)
		}
		defer eventLogger.Close()
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

	log.Infof("Listening on %s for private endpoints", cfg.privateListen)
	privListener, err := net.Listen("tcp", cfg.privateListen)
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

	log.Infof("Listening on %s", cfg.listen)
	listener, err := net.Listen("tcp", cfg.listen)
	if err != nil {
		log.Fatal(err)
	}

	r, err := routes(cfg, authenticator, ghIntegration, eventLogger)
	if err != nil {
		log.Fatal(err)
	}

	server := &graceful.Server{
		Timeout: cfg.stopTimeout,
		Server: &http.Server{
			Handler: r,
		},
	}
	var proxyListener net.Listener = &proxyproto.Listener{
		Listener:           listener,
		ProxyHeaderTimeout: proxyTimeout,
	}

	if cfg.redirectHTTPS {
		// We use a custom listener router to ensure only connections on port 443 get
		// through to the real router - everything else gets redirected.
		proxyListenerRouter := newProxyListenerRouter(proxyListener)
		proxyListener = proxyListenerRouter.listenerForPort(443)
		redirectServer := &http.Server{
			Handler: cfg.commonMiddleWare(nil).Wrap(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					url := r.URL
					if strings.HasSuffix(r.Host, ".weave.works") || strings.HasSuffix(r.Host, ".weave.works.") {
						url.Host = r.Host
					} else {
						url.Host = cfg.targetOrigin
					}
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
