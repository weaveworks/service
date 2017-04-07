package main

import (
	"flag"
	"fmt"
	"net/http"
	"time"
)

// Config is all the config we need to build the routes
type Config struct {
	apiInfo               string
	authCacheExpiration   time.Duration
	authCacheSize         int
	authHTTPURL           string
	authType              string
	authURL               string
	externalUI            bool
	fluentHost            string
	listen, privateListen string
	logLevel              string
	stopTimeout           time.Duration

	// Security-related flags
	hstsMaxAge    int
	redirectHTTPS bool
	secureCookie  bool
	sendCSPHeader bool
	targetOrigin  string

	// User-visible services - keep alphabetically sorted pls
	billingAPIHost      proxyConfig
	billingUIHost       proxyConfig
	collectionHost      proxyConfig
	configsHost         proxyConfig
	controlHost         proxyConfig
	demoHost            proxyConfig
	fluxHost            proxyConfig
	launchGeneratorHost proxyConfig
	peerDiscoveryHost   proxyConfig
	pipeHost            proxyConfig
	queryHost           proxyConfig
	uiMetricsHost       proxyConfig
	uiServerHost        proxyConfig

	// Admin services - keep alphabetically sorted pls
	alertmanagerHost      proxyConfig
	ansiblediffHost       proxyConfig
	billingAggregatorHost proxyConfig
	billingUploaderHost   proxyConfig
	compareImagesHost     proxyConfig
	devGrafanaHost        proxyConfig
	grafanaHost           proxyConfig
	kubedashHost          proxyConfig
	kubediffHost          proxyConfig
	lokiHost              proxyConfig
	prodGrafanaHost       proxyConfig
	promAlertmanagerHost  proxyConfig
	promDistributorHost   proxyConfig
	promQuerierHost       proxyConfig
	prometheusHost        proxyConfig
	scopeHost             proxyConfig
	terradiffHost         proxyConfig
	usersHost             proxyConfig
}

func (c *Config) proxies() map[string]*proxyConfig {
	return map[string]*proxyConfig{
		// User-visible services - keep alphabetically sorted pls.
		"billing-api":       &c.billingAPIHost,
		"billing-ui":        &c.billingUIHost,
		"collection":        &c.collectionHost,
		"configs":           &c.configsHost,
		"control":           &c.controlHost,
		"demo":              &c.demoHost,
		"flux":              &c.fluxHost,
		"launch-generator":  &c.launchGeneratorHost,
		"peer-discovery":    &c.peerDiscoveryHost,
		"pipe":              &c.pipeHost,
		"prom-alertmanager": &c.promAlertmanagerHost,
		"prom-distributor":  &c.promDistributorHost,
		"prom-querier":      &c.promQuerierHost,
		"query":             &c.queryHost,
		"ui-metrics":        &c.uiMetricsHost,
		"ui-server":         &c.uiServerHost,

		// Admin services - keep alphabetically sorted pls.
		"alertmanager":       &c.alertmanagerHost,
		"ansiblediff":        &c.ansiblediffHost,
		"billing-aggregator": &c.billingAggregatorHost,
		"billing-uploader":   &c.billingUploaderHost,
		"compare-images":     &c.compareImagesHost,
		"dev-grafana":        &c.devGrafanaHost,
		"grafana":            &c.grafanaHost,
		"kubedash":           &c.kubedashHost,
		"kubediff":           &c.kubediffHost,
		"loki":               &c.lokiHost,
		"prod-grafana":       &c.prodGrafanaHost,
		"prometheus":         &c.prometheusHost,
		"scope":              &c.scopeHost,
		"terradiff":          &c.terradiffHost,
		"users":              &c.usersHost,
	}
}

func (c *Config) RegisterFlags(f *flag.FlagSet) *Config {
	f.StringVar(&c.apiInfo, "api.info", "scopeservice:0.1", "Version info for the api to serve, in format ID:VERSION")
	f.DurationVar(&c.authCacheExpiration, "auth.cache.expiration", 30*time.Second, "How long to keep entries in the auth client.")
	f.IntVar(&c.authCacheSize, "auth.cache.size", 0, "How many entries to cache in the auth client.")
	f.StringVar(&c.authHTTPURL, "authenticator.httpurl", "http://users:80", "Where to find the authenticator's http service")
	f.StringVar(&c.authType, "authenticator", "web", "What authenticator to use: web | grpc | mock")
	f.StringVar(&c.authURL, "authenticator.url", "users:4772", "Where to find web the authenticator service")
	f.BoolVar(&c.externalUI, "externalUI", true, "Point to externally hosted static UI assets")
	f.StringVar(&c.fluentHost, "fluent", "", "Hostname & port for fluent")
	f.StringVar(&c.listen, "listen", ":80", "HTTP server listen address")
	f.StringVar(&c.privateListen, "private-listen", ":8080", "HTTP server listen address (private endpoints)")
	f.StringVar(&c.logLevel, "log.level", "info", "Logging level to use: debug | info | warn | error")
	f.DurationVar(&c.stopTimeout, "stop.timeout", 5*time.Second, "How long to wait for remaining requests to finish during shutdown")

	// Security-related flags
	f.IntVar(&c.hstsMaxAge, "hsts-max-age", 0, "Max Age in seconds for HSTS header - zero means no header.  Header will only be send if redirect-https is true.")
	f.BoolVar(&c.redirectHTTPS, "redirect-https", false, "Redirect all HTTP traffic to HTTPS")
	f.BoolVar(&c.secureCookie, "secure-cookie", false, "Send CRSF cookie as HTTPS only.")
	f.BoolVar(&c.sendCSPHeader, "send-csp-header", false, "Send \"Content-Security-Policy: default-src https:\" in all responses.")
	f.StringVar(&c.targetOrigin, "hostname", "", "Hostname through which this server is accessed, for same-origin checks (CSRF protection)")

	for name, proxyCfg := range c.proxies() {
		proxyCfg.name = name
		proxyCfg.RegisterFlags(f)
	}

	return c
}

type proxyConfig struct {
	// You should set this
	name string

	// Flag-set stuff
	hostAndPort string
	protocol    string
	readOnly    bool

	// Set this based on the flags
	http.Handler
}

func (p *proxyConfig) RegisterFlags(f *flag.FlagSet) {
	f.StringVar(&p.hostAndPort, p.name, "", fmt.Sprintf("Hostname & port for %s service", p.name))
	f.StringVar(&p.protocol, p.name+".protocol", "http", fmt.Sprintf("Protocol to connect to this %s service via (Must be: http or grpc)", p.name))
	f.BoolVar(&p.readOnly, p.name+".readonly", false, fmt.Sprintf("Make %s service, read-only (will only accept GETs)", p.name))
}
