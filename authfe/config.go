package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/weaveworks/service/common"
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
	gcpSSOSecret          string
	gcpWebhookSecret      string
	listen, privateListen string
	logLevel              string
	stopTimeout           time.Duration

	// Security-related flags
	hstsMaxAge            int
	redirectHTTPS         bool
	secureCookie          bool
	sendCSPHeader         bool
	targetOrigin          string
	allowedOriginSuffixes common.ArrayFlags
	cookieDomain          string

	// External hostnames
	launcherServiceExternalHost string

	// User-visible services - keep alphabetically sorted pls
	billingAPIHost         proxyConfig
	billingUIHost          proxyConfig
	collectionHost         proxyConfig
	controlHost            proxyConfig
	dashboardHost          proxyConfig
	demoHost               proxyConfig
	fluxHost               proxyConfig
	gcpServiceHost         proxyConfig
	gcpWebhookHost         proxyConfig
	githubReceiverHost     proxyConfig
	launchGeneratorHost    proxyConfig
	launcherServiceHost    proxyConfig
	notificationEventHost  proxyConfig
	notificationSenderHost proxyConfig
	peerDiscoveryHost      proxyConfig
	pipeHost               proxyConfig
	promConfigsHost        proxyConfig
	queryHost              proxyConfig
	uiMetricsHost          proxyConfig
	uiServerHost           proxyConfig

	// Admin services - keep alphabetically sorted pls
	alertmanagerHost      proxyConfig
	ansiblediffHost       proxyConfig
	billingAggregatorHost proxyConfig
	billingEnforcerHost   proxyConfig
	billingUploaderHost   proxyConfig
	compareImagesHost     proxyConfig
	compareRevisionsHost  proxyConfig
	corpTerradiffHost     proxyConfig
	devGrafanaHost        proxyConfig
	elasticsearchHost     proxyConfig
	eshHost               proxyConfig
	grafanaHost           proxyConfig
	jaegerHost            proxyConfig
	kibanaHost            proxyConfig
	kubedashHost          proxyConfig
	kubediffHost          proxyConfig
	lokiHost              proxyConfig
	notebooksHost         proxyConfig
	prodGrafanaHost       proxyConfig
	promAlertmanagerHost  proxyConfig
	promDistributorHost   proxyConfig
	promQuerierHost       proxyConfig
	prometheusHost        proxyConfig
	scopeHost             proxyConfig
	serviceUIKickerHost   proxyConfig
	terradiffHost         proxyConfig
	usersHost             proxyConfig
}

func (c *Config) proxies() map[string]*proxyConfig {
	return map[string]*proxyConfig{
		// User-visible services - keep alphabetically sorted pls.
		"billing-api":          &c.billingAPIHost,
		"billing-ui":           &c.billingUIHost,
		"collection":           &c.collectionHost,
		"control":              &c.controlHost,
		"dashboard":            &c.dashboardHost,
		"demo":                 &c.demoHost,
		"flux":                 &c.fluxHost,
		"gcp-launcher-webhook": &c.gcpWebhookHost,
		"gcp-service":          &c.gcpServiceHost,
		"launch-generator":     &c.launchGeneratorHost,
		"launcher-service":     &c.launcherServiceHost,
		"notebooks":            &c.notebooksHost,
		"notification-events":  &c.notificationEventHost,
		"notification-sender":  &c.notificationSenderHost,
		"peer-discovery":       &c.peerDiscoveryHost,
		"pipe":                 &c.pipeHost,
		"prom-alertmanager":    &c.promAlertmanagerHost,
		"prom-configs":         &c.promConfigsHost,
		"prom-distributor":     &c.promDistributorHost,
		"prom-querier":         &c.promQuerierHost,
		"query":                &c.queryHost,
		"ui-metrics":           &c.uiMetricsHost,
		"ui-server":            &c.uiServerHost,

		// Admin services - keep alphabetically sorted pls.
		"alertmanager":       &c.alertmanagerHost,
		"ansiblediff":        &c.ansiblediffHost,
		"billing-aggregator": &c.billingAggregatorHost,
		"billing-enforcer":   &c.billingEnforcerHost,
		"billing-uploader":   &c.billingUploaderHost,
		"compare-images":     &c.compareImagesHost,
		"compare-revisions":  &c.compareRevisionsHost,
		"corp-terradiff":     &c.corpTerradiffHost,
		"dev-grafana":        &c.devGrafanaHost,
		"elasticsearch":      &c.elasticsearchHost,
		"esh":                &c.eshHost,
		"grafana":            &c.grafanaHost,
		"jaeger":             &c.jaegerHost,
		"kibana":             &c.kibanaHost,
		"kubedash":           &c.kubedashHost,
		"kubediff":           &c.kubediffHost,
		"loki":               &c.lokiHost,
		"prod-grafana":       &c.prodGrafanaHost,
		"prometheus":         &c.prometheusHost,
		"scope":              &c.scopeHost,
		"service-ui-kicker":  &c.serviceUIKickerHost,
		"terradiff":          &c.terradiffHost,
		"users":              &c.usersHost,

		// For backwards compatibility - remove this once command line flags are updated.
		// Giving the same pointer to flag twice will make them aliases of each other.
		"configs": &c.promConfigsHost,
	}
}

// RegisterFlags registers all the authfe flags with a flagset
func (c *Config) RegisterFlags(f *flag.FlagSet) {
	f.StringVar(&c.apiInfo, "api.info", "scopeservice:0.1", "Version info for the api to serve, in format ID:VERSION")
	f.DurationVar(&c.authCacheExpiration, "auth.cache.expiration", 30*time.Second, "How long to keep entries in the auth client.")
	f.IntVar(&c.authCacheSize, "auth.cache.size", 0, "How many entries to cache in the auth client.")
	f.StringVar(&c.authHTTPURL, "authenticator.httpurl", "http://users:80", "Where to find the authenticator's http service")
	f.StringVar(&c.authType, "authenticator", "web", "What authenticator to use: web | grpc | mock")
	f.StringVar(&c.authURL, "authenticator.url", "users:4772", "Where to find the authenticator service")
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
	f.Var(&c.allowedOriginSuffixes, "allowed-origin-suffix", "Hostname suffix to permit through same-origin checks (CSRF protection).")
	f.StringVar(&c.cookieDomain, "cookie-domain", "", "Domain to which cookies will be scoped")

	// External hostnames
	f.StringVar(&c.launcherServiceExternalHost, "launcher-service-external-host", "get.weave.works", "External hostname used for the launcher service")

	for name, proxyCfg := range c.proxies() {
		proxyCfg.RegisterFlags(name, f)
	}

	// Deprecated
	_ = f.String("flux-v6", "", "deprecated: set -flux to point to the flux-api service instead")
}

// ReadEnvVars loads environment variables.
func (c *Config) ReadEnvVars() {
	c.gcpWebhookSecret = os.Getenv("GCP_LAUNCHER_WEBHOOK_SECRET") // Secret used to authenticate incoming GCP webhook requests.
	c.gcpSSOSecret = os.Getenv("GCP_LAUNCHER_SSO_SECRET")         // Secret used to authenticate incoming GCP SSO requests.
}

type proxyConfig struct {
	// Determines the names of the flags
	name string

	// Values set by flags.
	hostAndPort string
	protocol    string
	readOnly    bool
	grpcHost    string

	// Set this based on the flags
	http.Handler
}

func (p *proxyConfig) RegisterFlags(name string, f *flag.FlagSet) {
	p.name = name
	f.StringVar(&p.hostAndPort, name, "", fmt.Sprintf("Hostname & port for %s service", name))
	f.StringVar(&p.protocol, name+".protocol", "http", fmt.Sprintf("Protocol to connect to this %s service via (Must be: http or grpc)", name))
	f.BoolVar(&p.readOnly, name+".readonly", false, fmt.Sprintf("Make %s service, read-only (will only accept GETs)", name))
	f.StringVar(&p.grpcHost, name+"-grpc", "", fmt.Sprintf("Use gRPC for %s, instead of protocol (backwards compat)", name))
}
