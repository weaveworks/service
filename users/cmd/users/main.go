package main

import (
	"context"
	"flag"
	"math/rand"
	"time"

	"github.com/FrenchBen/goketo"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"

	"github.com/weaveworks/common/logging"
	"github.com/weaveworks/common/server"
	"github.com/weaveworks/common/tracing"
	"github.com/weaveworks/service/common"
	billing_grpc "github.com/weaveworks/service/common/billing/grpc"
	"github.com/weaveworks/service/common/dbconfig"
	"github.com/weaveworks/service/common/featureflag"
	"github.com/weaveworks/service/common/gcp/procurement"
	"github.com/weaveworks/service/users"
	users_sync "github.com/weaveworks/service/users-sync/api"
	"github.com/weaveworks/service/users/api"
	"github.com/weaveworks/service/users/db"
	"github.com/weaveworks/service/users/emailer"
	grpc_server "github.com/weaveworks/service/users/grpc"
	"github.com/weaveworks/service/users/login"
	"github.com/weaveworks/service/users/marketing"
	"github.com/weaveworks/service/users/render"
	"github.com/weaveworks/service/users/sessions"
	"github.com/weaveworks/service/users/templates"
)

func init() {
	prometheus.MustRegister(common.DatabaseRequestDuration)
	prometheus.MustRegister(api.ServiceStatusRequestDuration)
}

func main() {

	traceCloser := tracing.NewFromEnv("users")
	defer traceCloser.Close()

	serverConfig := server.Config{
		MetricsNamespace: common.PrometheusNamespace,
		GRPCMiddleware:   []grpc.UnaryServerInterceptor{render.GRPCErrorInterceptor},
	}
	serverConfig.RegisterFlags(flag.CommandLine)
	flag.CommandLine.IntVar(&serverConfig.HTTPListenPort, "port", 80, "HTTP port to listen on")
	flag.CommandLine.IntVar(&serverConfig.GRPCListenPort, "grpc-port", 4772, "gRPC port to listen on")
	var (
		domain           = flag.String("domain", "https://cloud.weave.works", "domain where scope service is runnning.")
		emailURI         = flag.String("email-uri", "", "uri of smtp server to send email through, of the format: smtp://username:password@hostname:port.  Email-uri must be provided. For local development, you can set this to: log://, which will log all emails.")
		sessionSecret    = flag.String("session-secret", "XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX", "Secret used validate sessions")
		createAdminUsers = flag.Bool("create-admin-users-on-demand", false, "When a user tries to sign up with their email, create an admin account of that user (DEV only)")
		secureCookie     = flag.Bool("secure-cookie", false, "Set secure flag on cookies (so they only get used on HTTPS connections.)")
		cookieDomain     = flag.String("cookie-domain", "", "The domain to which the authentication cookie will be scoped.")

		fluxStatusAPI  = flag.String("flux-status-api", "", "Hostname and port for flux V6 service. e.g. http://fluxsvc.flux.svc.cluster.local:80/api/flux/v6/status")
		scopeProbesAPI = flag.String("scope-probes-api", "", "Hostname and port for scope query. e.g. http://query.scope.svc.cluster.local:80/api/probes")
		promMetricsAPI = flag.String("prom-metrics-api", "", "Hostname and port for cortex querier. e.g. http://querier.cortex.svc.cluster.local:80/api/prom/api/v1/label/__name__/values")
		cortexStatsAPI = flag.String("cortex-stats-api", "", "Hostname and port for cortex stats. e.g. http://querier.cortex.svc.cluster.local:80/api/prom/user_stats")
		netPeersAPI    = flag.String("net-peers-api", "", "Hostname and port for peer discovery. e.g. http://discovery.service-net.svc.cluster.local:80/api/net/peers")

		marketoMunchkinKey = flag.String("marketo-munchkin-key", "", "Secret key for Marketo munchkin.")
		intercomHashKey    = flag.String("intercom-hash-key", "", "Secret key for Intercom user hash.")
		mixpanelToken      = flag.String("mixpanel-token", "", "Mixpanel project API token")

		emailFromAddress = flag.String("email-from-address", "Weave Cloud <support@weave.works>", "From address for emails.")

		forceFeatureFlags common.ArrayFlags
		webhookTokens     common.ArrayFlags

		billingFeatureFlagProbability = flag.Uint("billing-feature-flag-probability", 0, "Percentage of *new* organizations for which we want to enable the 'billing' feature flag. 0 means always disabled. 100 means always enabled. Any value X in between will enable billing randomly X% of the time.")

		dbCfg          dbconfig.Config
		procurementCfg procurement.Config
		billingCfg     billing_grpc.Config
		usersSyncCfg   users_sync.Config
		marketoCfg     marketing.MarketoConfig

		cleanupURLs common.ArrayFlags

		notificationReceiversURL = flag.String("notification-receivers-url", "http://eventmanager.notification.svc.cluster.local/api/notification/config/receivers", "Notification service URL for creating receivers")
	)

	flag.Var(&forceFeatureFlags, "force-feature-flags", "Force this feature flag to be on for all organisations.")
	flag.Var(&forceFeatureFlags, "fff", "Force this feature flag to be on for all organisations.")
	flag.Var(&webhookTokens, "webhook-token", "Secret tokens used to validate webhooks from external services (e.g. Marketo).")
	flag.Var(&cleanupURLs, "cleanup-url", "Endpoints for cleanup after instance deletion")

	logins := login.NewAuth0Provider()
	logins.Flags(flag.CommandLine)
	logins.Register("google", login.NewGoogleConnection())
	logins.Register("github", login.NewGithubConnection())
	logins.Register("email", login.NewEmailConnection())
	dbCfg.RegisterFlags(flag.CommandLine, "postgres://postgres@users-db.weave.local/users?sslmode=disable", "URI where the database can be found (for dev you can use memory://)", "/migrations", "Migrations directory.")
	procurementCfg.RegisterFlags(flag.CommandLine)
	billingCfg.RegisterFlags(flag.CommandLine)
	usersSyncCfg.RegisterFlags(flag.CommandLine)
	marketoCfg.RegisterFlags(flag.CommandLine)

	flag.Parse()

	if err := logins.SetSiteDomain(*domain); err != nil {
		log.Fatalf("Error setting up login: %v", err)
	}

	if err := logging.Setup(serverConfig.LogLevel.String()); err != nil {
		log.Fatalf("Error configuring logging: %v", err)
		return
	}
	serverConfig.Log = logging.Logrus(log.StandardLogger())

	var billingEnabler featureflag.Enabler
	billingEnabler = featureflag.NewRandomEnabler(*billingFeatureFlagProbability)
	log.Infof("Billing enabled for %v%% of newly created organizations.", *billingFeatureFlagProbability)

	billingClient, err := billing_grpc.NewClient(billingCfg)
	if err != nil {
		log.Fatalf("Failed creating billing-api's gRPC client: %v", err)
	}
	defer billingClient.Close()

	usersSyncClient, err := users_sync.NewClient(usersSyncCfg)
	if err != nil {
		log.Fatalf("Failed creating users-sync-api's gRPC client: %v", err)
	}
	defer usersSyncClient.Close()

	var marketingQueues marketing.Queues
	if marketoCfg.ClientID != "" {
		goketoClient, err := goketo.NewAuthClient(marketoCfg.ClientID, marketoCfg.Secret, marketoCfg.Endpoint)
		if err != nil {
			log.Warningf("Failed to initialise Marketo client: %v", err)
		} else {
			marketoClient := marketing.NewMarketoClient(goketoClient, marketoCfg.Program)
			queue := marketing.NewQueue(marketoClient)
			defer queue.Stop()
			marketingQueues = append(marketingQueues, queue)
		}
	}

	var mixpanelClient *marketing.MixpanelClient
	if *mixpanelToken != "" {
		mixpanelClient = marketing.NewMixpanelClient(*mixpanelToken)
	}

	var procurementClient procurement.API
	if procurementCfg.ServiceAccountKeyFile != "" {
		var err error
		procurementClient, err = procurement.NewClient(procurementCfg)
		if err != nil {
			log.Fatalf("Failed creating Google Partner Procurement API client: %v", err)
		}
	}

	webhookTokenMap := make(map[string]struct{})
	for _, value := range webhookTokens {
		webhookTokenMap[value] = struct{}{}
	}

	rand.Seed(time.Now().UnixNano())

	templates := templates.MustNewEngine("templates")
	emailer := emailer.MustNew(*emailURI, *emailFromAddress, templates, *domain)
	db := db.MustNew(dbCfg)
	defer db.Close(context.Background())
	sessions := sessions.MustNewStore(*sessionSecret, *secureCookie, *cookieDomain)

	log.Debug("Debug logging enabled")

	grpcServer := grpc_server.New(sessions, db, emailer, marketingQueues, forceFeatureFlags)
	app := api.New(
		*createAdminUsers,
		emailer,
		sessions,
		db,
		&logins,
		templates,
		marketingQueues,
		forceFeatureFlags,
		*marketoMunchkinKey,
		*intercomHashKey,
		grpcServer,
		webhookTokenMap,
		mixpanelClient,
		procurementClient,
		*fluxStatusAPI,
		*scopeProbesAPI,
		*promMetricsAPI,
		*cortexStatsAPI,
		*netPeersAPI,
		billingClient,
		billingEnabler,
		*notificationReceiversURL,
		usersSyncClient,
	)

	log.Infof("Listening on ports %d (HTTP) and %d (gRPC)", serverConfig.HTTPListenPort, serverConfig.GRPCListenPort)
	s, err := server.New(serverConfig)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
		return
	}
	defer s.Shutdown()

	app.RegisterRoutes(s.HTTP)
	users.RegisterUsersServer(s.GRPC, grpcServer)
	s.Run()
}
