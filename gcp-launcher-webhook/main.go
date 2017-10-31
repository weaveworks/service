package main

import (
	"context"
	"flag"
	"time"

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
	port     int
	endpoint string
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

func main() {
	var cfg config
	cfg.RegisterFlags(flag.CommandLine)
	flag.Parse()

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

	pub, err := publisher.New(context.Background(), cfg.publisher)
	if err != nil {
		log.Fatalf("Failed creating Pub/Sub publisher: %v", err)
	}
	defer pub.Close()
	sub, err := pub.CreateSubscription(cfg.subscriptionID, cfg.endpoint, 10*time.Second)
	if err != nil {
		log.Fatalf("Failed subscribing to Pub/Sub topic: %v", err)
	}

	log.Infof("Subscription [%s] is active, awaiting messages at: %v", sub, cfg.endpoint)

	server.HTTP.Handle(
		"/",
		webhook.New(&subscription.MessageHandler{
			Partner: partner,
			Users:   users,
		}),
	).Methods("POST").Name("webhook")
	server.Run()
}
