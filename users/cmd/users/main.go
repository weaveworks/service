package main

import (
	"context"
	"flag"
	"math/rand"
	"strings"
	"time"

	"github.com/FrenchBen/goketo"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"

	"github.com/weaveworks/common/logging"
	"github.com/weaveworks/common/server"
	"github.com/weaveworks/service/common"
	billing_grpc "github.com/weaveworks/service/common/billing/grpc"
	"github.com/weaveworks/service/common/dbconfig"
	"github.com/weaveworks/service/common/featureflag"
	"github.com/weaveworks/service/common/gcp/partner"
	"github.com/weaveworks/service/common/tracing"
	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/api"
	"github.com/weaveworks/service/users/cleaner"
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

	traceCloser := tracing.Init("users")
	defer traceCloser.Close()

	var (
		logLevel      = flag.String("log.level", "info", "Logging level to use: debug | info | warn | error")
		port          = flag.Int("port", 80, "port to listen on")
		grpcPort      = flag.Int("grpc-port", 4772, "grpc port to listen on")
		domain        = flag.String("domain", "https://cloud.weave.works", "domain where scope service is runnning.")
		emailURI      = flag.String("email-uri", "", "uri of smtp server to send email through, of the format: smtp://username:password@hostname:port.  Email-uri must be provided. For local development, you can set this to: log://, which will log all emails.")
		sessionSecret = flag.String("session-secret", "XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX", "Secret used validate sessions")
		directLogin   = flag.Bool("direct-login", false, "Send login token in the signup response (DEV only)")
		secureCookie  = flag.Bool("secure-cookie", false, "Set secure flag on cookies (so they only get used on HTTPS connections.)")

		fluxStatusAPI  = flag.String("flux-status-api", "", "Hostname and port for flux V6 service. e.g. http://fluxsvc.flux.svc.cluster.local:80/api/flux/v6/status")
		scopeProbesAPI = flag.String("scope-probes-api", "", "Hostname and port for scope query. e.g. http://query.scope.svc.cluster.local:80/api/probes")
		promMetricsAPI = flag.String("prom-metrics-api", "", "Hostname and port for cortex querier. e.g. http://querier.cortex.svc.cluster.local:80/api/prom/api/v1/label/__name__/values")
		cortexStatsAPI = flag.String("cortex-stats-api", "", "Hostname and port for cortex stats. e.g. http://querier.cortex.svc.cluster.local:80/api/prom/user_stats")
		netPeersAPI    = flag.String("net-peers-api", "", "Hostname and port for peer discovery. e.g. http://discovery.service-net.svc.cluster.local:80/api/net/peers")

		marketoClientID    = flag.String("marketo-client-id", "", "Client ID of Marketo account.  If not supplied marketo integration will be disabled.")
		marketoSecret      = flag.String("marketo-secret", "", "Secret for Marketo account.")
		marketoEndpoint    = flag.String("marketo-endpoint", "", "REST API endpoint for Marketo.")
		marketoProgram     = flag.String("marketo-program", "2016_00_Website_WeaveCloud", "Program name to add leads to (for Marketo).")
		marketoMunchkinKey = flag.String("marketo-munchkin-key", "", "Secret key for Marketo munchkin.")
		intercomHashKey    = flag.String("intercom-hash-key", "", "Secret key for Intercom user hash.")
		mixpanelToken      = flag.String("mixpanel-token", "", "Mixpanel project API token")

		emailFromAddress = flag.String("email-from-address", "Weave Cloud <support@weave.works>", "From address for emails.")

		localTestUserCreate               = flag.Bool("local-test-user.create", false, "Create a test user (for local deployments only.)")
		localTestUserEmail                = flag.String("local-test-user.email", "test@test.test", "Email for test user (for local deployments only.)")
		localTestUserInstanceID           = flag.String("local-test-user.instance-id", "local-test", "Instance ID for test user (for local deployments only.)")
		localTestUserInstanceName         = flag.String("local-test-user.instance-name", "Local Test Instance", "Instance name for test user (for local deployments only.)")
		localTestUserInstanceToken        = flag.String("local-test-user.instance-token", "local-test-token", "Instance token for test user (for local deployments only.)")
		localTestUserInstanceFeatureFlags = flag.String("local-test-user.instance-feature-flags", "", "Comma-separated feature flags for the test user (for local deployments only.)")

		forceFeatureFlags common.ArrayFlags
		webhookTokens     common.ArrayFlags

		billingFeatureFlagProbability = flag.Uint("billing-feature-flag-probability", 0, "Percentage of *new* organizations for which we want to enable the 'billing' feature flag. 0 means always disabled. 100 means always enabled. Any value X in between will enable billing randomly X% of the time.")

		dbCfg      dbconfig.Config
		partnerCfg partner.Config
		billingCfg billing_grpc.Config

		cleanupURLs common.ArrayFlags
	)

	flag.Var(&forceFeatureFlags, "force-feature-flags", "Force this feature flag to be on for all organisations.")
	flag.Var(&forceFeatureFlags, "fff", "Force this feature flag to be on for all organisations.")
	flag.Var(&webhookTokens, "webhook-token", "Secret tokens used to validate webhooks from external services (e.g. Marketo).")
	flag.Var(&cleanupURLs, "cleanup-url", "Endpoints for cleanup after instance deletion")

	logins := login.NewProviders()
	logins.Register(login.GithubProviderID, login.NewGithubProvider())
	logins.Register(login.GoogleProviderID, login.NewGoogleProvider())
	logins.Flags(flag.CommandLine)
	dbCfg.RegisterFlags(flag.CommandLine, "postgres://postgres@users-db.weave.local/users?sslmode=disable", "URI where the database can be found (for dev you can use memory://)", "/migrations", "Migrations directory.")
	partnerCfg.RegisterFlags(flag.CommandLine)
	billingCfg.RegisterFlags(flag.CommandLine)

	partnerAccess := partner.NewAccess()
	partnerAccess.Flags(flag.CommandLine)

	flag.Parse()

	if err := logging.Setup(*logLevel); err != nil {
		log.Fatalf("Error configuring logging: %v", err)
		return
	}

	var billingEnabler featureflag.Enabler
	billingEnabler = featureflag.NewRandomEnabler(*billingFeatureFlagProbability)
	log.Infof("Billing enabled for %v%% of newly created organizations.", *billingFeatureFlagProbability)

	billingClient, err := billing_grpc.NewClient(billingCfg)
	if err != nil {
		log.Fatalf("Failed creating billing-api's gRPC client: %v", err)
	}
	defer billingClient.Close()

	var marketingQueues marketing.Queues
	if *marketoClientID != "" {
		goketoClient, err := goketo.NewAuthClient(*marketoClientID, *marketoSecret, *marketoEndpoint)
		if err != nil {
			log.Warningf("Failed to initialise Marketo client: %v", err)
		} else {
			marketoClient := marketing.NewMarketoClient(goketoClient, *marketoProgram)
			queue := marketing.NewQueue(marketoClient)
			defer queue.Stop()
			marketingQueues = append(marketingQueues, queue)
		}
	}

	var mixpanelClient *marketing.MixpanelClient
	if *mixpanelToken != "" {
		mixpanelClient = marketing.NewMixpanelClient(*mixpanelToken)
	}

	var partnerClient partner.API
	if partnerCfg.ServiceAccountKeyFile != "" {
		var err error
		partnerClient, err = partner.NewClient(partnerCfg)
		if err != nil {
			log.Fatalf("Failed creating Google Partner Subscriptions API client: %v", err)
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
	sessions := sessions.MustNewStore(*sessionSecret, *secureCookie)

	orgCleaner := cleaner.New(cleanupURLs, db)
	log.Debug("Debug logging enabled")

	grpcServer := grpc_server.New(sessions, db, emailer)
	app := api.New(
		*directLogin,
		emailer,
		sessions,
		db,
		logins,
		templates,
		marketingQueues,
		forceFeatureFlags,
		*marketoMunchkinKey,
		*intercomHashKey,
		grpcServer,
		webhookTokenMap,
		mixpanelClient,
		partnerClient,
		partnerAccess,
		*fluxStatusAPI,
		*scopeProbesAPI,
		*promMetricsAPI,
		*cortexStatsAPI,
		*netPeersAPI,
		billingClient,
		billingEnabler,
		orgCleaner,
	)

	if *localTestUserCreate {
		makeLocalTestUser(app, *localTestUserEmail, *localTestUserInstanceID,
			*localTestUserInstanceName, *localTestUserInstanceToken,
			strings.Split(*localTestUserInstanceFeatureFlags, ","))
	}

	log.Infof("Listening on port %d", *port)
	s, err := server.New(server.Config{
		MetricsNamespace:        common.PrometheusNamespace,
		HTTPListenPort:          *port,
		GRPCListenPort:          *grpcPort,
		GRPCMiddleware:          []grpc.UnaryServerInterceptor{render.GRPCErrorInterceptor},
		RegisterInstrumentation: true,
	})
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
		return
	}
	defer s.Shutdown()

	app.RegisterRoutes(s.HTTP)
	users.RegisterUsersServer(s.GRPC, grpcServer)

	ctx, cancel := context.WithCancel(context.Background())
	app.OrgCleaner.Run(ctx)
	s.Run()
	cancel()
}

func makeLocalTestUser(a *api.API, email, instanceID, instanceName, token string, featureFlags []string) {
	ctx := context.Background()
	_, user, err := a.Signup(ctx, api.SignupRequest{
		Email:       email,
		QueryParams: make(map[string]string)})
	if err != nil {
		log.Errorf("Error creating local test user: %v", err)
		return
	}

	if err := a.UpdateUserAtLogin(ctx, user); err != nil {
		log.Errorf("Error updating user first login at: %v", err)
		return
	}

	if err := a.MakeUserAdmin(ctx, user.ID, true); err != nil {
		log.Errorf("Error making user an admin: %v", err)
		return
	}

	if err := a.CreateOrg(ctx, user, api.OrgView{
		ExternalID:     instanceID,
		Name:           instanceName,
		ProbeToken:     token,
		FeatureFlags:   featureFlags,
		TrialExpiresAt: user.TrialExpiresAt(),
	}, time.Now()); err != nil {
		log.Errorf("Error creating local test instance: %v", err)
		return
	}
}
