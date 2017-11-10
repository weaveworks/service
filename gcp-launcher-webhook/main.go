package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"

	"github.com/weaveworks/common/server"
	"github.com/weaveworks/service/common"
	"github.com/weaveworks/service/common/gcp/partner"
	"github.com/weaveworks/service/common/gcp/pubsub/publisher"
	"github.com/weaveworks/service/common/gcp/pubsub/webhook"
	"github.com/weaveworks/service/common/users"
	"github.com/weaveworks/service/gcp-launcher-webhook/subscription"
)

type config struct {
	port           int
	endpoint       string
	secret         string // Secret used to authenticate incoming GCP webhook requests.
	subscriptionID string

	publisher publisher.Config

	users   users.Config
	partner partner.Config
}

func (c *config) RegisterFlags(f *flag.FlagSet) {
	flag.IntVar(&c.port, "port", 80, "HTTP port for the Cloud Launcher's GCP Pub/Sub push webhook")
	flag.StringVar(&c.endpoint, "webhook-endpoint", "https://frontend.dev.weave.works/api/gcp-launcher/webhook", "Endpoint this webhook is accessible from the outside")
	flag.StringVar(&c.subscriptionID, "pubsub-api.subscription-id", "gcp-subscriptions", "Arbitrary name that denotes this subscription")

	c.publisher.RegisterFlags(f)
	c.partner.RegisterFlags(f)
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

	createSubscription(&cfg)

	users, err := users.NewClient(cfg.users)
	if err != nil {
		log.Fatalf("Failed initialising users client: %v", err)
	}

	partner, err := partner.NewClient(cfg.partner)
	if err != nil {
		log.Fatalf("Failed creating Google Partner Subscriptions API client: %v", err)
	}

	serverCfg := server.Config{
		HTTPListenPort:          cfg.port,
		MetricsNamespace:        common.PrometheusNamespace,
		RegisterInstrumentation: true,
	}
	server, err := server.New(serverCfg)
	if err != nil {
		log.Fatalf("Failed to start GCP Cloud Launcher webhook: %v", err)
	}
	defer server.Shutdown()

	server.HTTP.Handle(
		"/",
		webhook.New(&subscription.MessageHandler{
			Partner: partner,
			Users:   users,
		}),
	).Methods("POST").Name("webhook")
	log.Infof("Starting GCP Cloud Launcher webhook...")
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
