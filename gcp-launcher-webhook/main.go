package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"

	"github.com/weaveworks/common/logging"
	"github.com/weaveworks/common/server"
	"github.com/weaveworks/service/common"
	"github.com/weaveworks/service/common/gcp/procurement"
	"github.com/weaveworks/service/common/gcp/pubsub/dto"
	"github.com/weaveworks/service/common/gcp/pubsub/publisher"
	"github.com/weaveworks/service/common/gcp/pubsub/webhook"
	"github.com/weaveworks/service/common/users"
	"github.com/weaveworks/service/gcp-launcher-webhook/entitlement"
)

type config struct {
	port                   int
	endpoint               string
	logLevel               string
	secret                 string // Secret used to authenticate incoming GCP webhook requests.
	createSubscription     bool
	createSubscriptionPull bool
	subscriptionID         string

	publisher publisher.Config

	users       users.Config
	procurement procurement.Config
}

func (c *config) RegisterFlags(f *flag.FlagSet) {
	flag.StringVar(&c.logLevel, "log.level", "info", "Logging level to use: debug | info | warn | error")
	flag.IntVar(&c.port, "port", 80, "HTTP port for the Cloud Launcher's GCP Pub/Sub push webhook")
	flag.StringVar(&c.endpoint, "webhook-endpoint", "https://frontend.dev.weave.works/api/gcp-launcher/webhook?secret=FILLMEIN", "Endpoint this webhook is accessible from the outside")
	flag.BoolVar(&c.createSubscription, "pubsub-api.create-subscription", false, "Enable/Disable programmatic creation of the Pub/Sub subscription.")
	flag.BoolVar(&c.createSubscriptionPull, "pubsub-api.create-subscription-pull", false, "(dev only!) Whether to createSubscriptionPull message rather than push")
	flag.StringVar(&c.subscriptionID, "pubsub-api.subscription-id", "gcp-subscriptions", "Arbitrary name that denotes the Pub/Sub subscription 'pushing' to this webhook.")

	c.publisher.RegisterFlags(f)
	c.procurement.RegisterFlags(f)
	c.users.RegisterFlags(f)
}

func (c *config) ReadEnvVars() {
	c.secret = os.Getenv("GCP_LAUNCHER_WEBHOOK_SECRET") // Secret used to authenticate incoming GCP webhook requests.
}

func (c config) Endpoint() string {
	if len(c.secret) > 0 {
		return fmt.Sprintf("%v?secret=%v", c.endpoint, c.secret)
	}
	return c.endpoint
}

func main() {
	var cfg config
	cfg.ReadEnvVars()
	cfg.RegisterFlags(flag.CommandLine)
	flag.Parse()

	if err := logging.Setup(cfg.logLevel); err != nil {
		log.Fatalf("Error configuring logging: %v", err)
		return
	}

	users, err := users.NewClient(cfg.users)
	if err != nil {
		log.Fatalf("Failed initialising users client: %v", err)
	}

	procurement, err := procurement.NewClient(cfg.procurement)
	if err != nil {
		log.Fatalf("Failed creating Google Partner Procurement API client: %v", err)
	}
	handler := &entitlement.MessageHandler{
		Procurement: procurement,
		Users:       users,
	}

	serverCfg := server.Config{
		HTTPListenPort:          cfg.port,
		MetricsNamespace:        common.PrometheusNamespace,
		RegisterInstrumentation: true,
		Log:                     logging.Logrus(log.StandardLogger()),
	}
	server, err := server.New(serverCfg)
	if err != nil {
		log.Fatalf("Failed to start GCP Cloud Launcher webhook: %v", err)
	}
	defer server.Shutdown()

	if cfg.createSubscriptionPull {
		createSubscriptionCallback(&cfg, handler.Handle)
	}
	if cfg.createSubscription {
		createSubscription(&cfg)
		server.HTTP.Handle("/", webhook.New(handler)).Methods("POST").Name("webhook")
	}

	log.Infof("Starting server")
	server.Run()
}

// createSubscription programmatically creates a GCP Pub/Sub subscription, signaling GCP Pub/Sub to "push" to our webhook.
// IMPORTANT:
// With more than one replica of this service, we might run into race conditions when creating the subscription.
// If/when this happens, we may want to consider either manually setting the subscription up in the GCP portal, or using Terraform to do it.
func createSubscription(cfg *config) {
	log.Infof("Creating GCP Pub/Sub subscription [projects/%v/subscriptions/%v]...", cfg.publisher.ProjectID, cfg.subscriptionID)
	pub, err := publisher.New(context.Background(), cfg.publisher)
	if err != nil {
		log.Fatalf("Failed creating Pub/Sub publisher: %v", err)
	}
	defer pub.Close()
	sub, err := pub.CreateSubscription(cfg.subscriptionID, cfg.Endpoint(), cfg.publisher.AckDeadline)
	if err != nil {
		log.Fatalf("Failed subscribing to Pub/Sub topic: %v", err)
	}
	log.Infof("Created subscription [%s], awaiting messages at: %v", sub, cfg.Endpoint())
}

func createSubscriptionCallback(cfg *config, callback func(msg dto.Message) error) {
	log.Infof("Creating GCP Pub/Sub callback subscription [projects/%v/subscriptions/%v]", cfg.publisher.ProjectID, cfg.subscriptionID)
	pub, err := publisher.New(context.Background(), cfg.publisher)
	if err != nil {
		log.Fatalf("Failed creating Pub/Sub publisher: %v", err)
	}
	go func() {
		defer pub.Close()
		err := pub.CreateSubscriptionCallback(cfg.subscriptionID, cfg.publisher.AckDeadline, callback)
		if err != nil {
			log.Fatalf(err.Error())
		}
	}()
}
