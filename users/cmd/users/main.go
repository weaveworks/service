package main

import (
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/tylerb/graceful"

	"github.com/weaveworks/service/common"
	"github.com/weaveworks/service/common/logging"
	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/api"
	"github.com/weaveworks/service/users/db"
	"github.com/weaveworks/service/users/emailer"
	"github.com/weaveworks/service/users/login"
	"github.com/weaveworks/service/users/marketing"
	"github.com/weaveworks/service/users/sessions"
	"github.com/weaveworks/service/users/templates"
)

func main() {
	var (
		logLevel           = flag.String("log.level", "info", "Logging level to use: debug | info | warn | error")
		logSuccess         = flag.Bool("log.success", false, "Log successful requests.")
		port               = flag.Int("port", 80, "port to listen on")
		stopTimeout        = flag.Duration("stop.timeout", 5*time.Second, "How long to wait for remaining requests to finish during shutdown")
		domain             = flag.String("domain", "https://cloud.weave.works", "domain where scope service is runnning.")
		databaseURI        = flag.String("database-uri", "postgres://postgres@users-db.weave.local/users?sslmode=disable", "URI where the database can be found (for dev you can use memory://)")
		databaseMigrations = flag.String("database-migrations", "", "Path where the database migration files can be found")
		emailURI           = flag.String("email-uri", "", "uri of smtp server to send email through, of the format: smtp://username:password@hostname:port.  Email-uri must be provided. For local development, you can set this to: log://, which will log all emails.")
		sessionSecret      = flag.String("session-secret", "XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX", "Secret used validate sessions")
		directLogin        = flag.Bool("direct-login", false, "Send login token in the signup response (DEV only)")

		pardotEmail    = flag.String("pardot-email", "", "Email of Pardot account.  If not supplied pardot integration will be disabled.")
		pardotPassword = flag.String("pardot-password", "", "Password of Pardot account.")
		pardotUserKey  = flag.String("pardot-userkey", "", "User key of Pardot account.")

		marketoClientID = flag.String("marketo-client-id", "", "Client ID of Marketo account.  If not supplied marketo integration will be disabled.")
		marketoSecret   = flag.String("marketo-secret", "", "Secret for Marketo account.")
		marketoEndpoint = flag.String("marketo-endpoint", "", "REST API endpoint for Marketo.")

		emailFromAddress = flag.String("email-from-address", "Weave Cloud <support@weave.works>", "From address for emails.")

		forceFeatureFlags common.ArrayFlags
	)

	flag.Var(&forceFeatureFlags, "force-feature-flags", "Force this feature flag to be on for all organisations.")
	flag.Var(&forceFeatureFlags, "fff", "Force this feature flag to be on for all organisations.")

	logins := login.NewProviders()
	logins.Register("github", login.NewGithubProvider())
	logins.Register("google", login.NewGoogleProvider())
	logins.Flags(flag.CommandLine)

	flag.Parse()

	if err := logging.Setup(*logLevel); err != nil {
		logrus.Fatalf("Error configuring logging: %v", err)
		return
	}

	var marketingQueues marketing.Queues
	if *pardotEmail != "" {
		pardotClient := marketing.NewPardotClient(marketing.PardotAPIURL,
			*pardotEmail, *pardotPassword, *pardotUserKey)
		queue := marketing.NewQueue(pardotClient)
		defer queue.Stop()
		marketingQueues = append(marketingQueues, queue)
	}

	if *marketoClientID != "" {
		marketoClient, err := marketing.NewMarketoClient(*marketoClientID, *marketoSecret, *marketoEndpoint)
		if err != nil {
			logrus.Warningf("Failed to initialise Marketo client: %v", err)
		} else {
			queue := marketing.NewQueue(marketoClient)
			defer queue.Stop()
			marketingQueues = append(marketingQueues, queue)
		}
	}

	rand.Seed(time.Now().UnixNano())

	templates := templates.MustNewEngine("templates")
	emailer := emailer.MustNew(*emailURI, *emailFromAddress, templates, *domain)
	db := db.MustNew(*databaseURI, *databaseMigrations)
	defer db.Close()
	sessions := sessions.MustNewStore(*sessionSecret)

	logrus.Debug("Debug logging enabled")

	logrus.Infof("Listening on port %d", *port)
	mux := http.NewServeMux()

	mux.Handle("/", api.New(*directLogin, *logSuccess, emailer, sessions, db, logins, templates, marketingQueues, forceFeatureFlags))
	mux.Handle("/metrics", makePrometheusHandler())
	if err := graceful.RunWithErr(fmt.Sprintf(":%d", *port), *stopTimeout, mux); err != nil {
		logrus.Fatal(err)
	}
	logrus.Info("Gracefully shut down")
}

func makePrometheusHandler() http.Handler {
	prometheus.MustRegister(users.RequestDuration)
	prometheus.MustRegister(users.DatabaseRequestDuration)
	return prometheus.Handler()
}
