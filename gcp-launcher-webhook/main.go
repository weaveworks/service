package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

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
	endpoint           string
	logLevel           string
	secret             string // Secret used to authenticate incoming GCP webhook requests.
	createSubscription bool
	pull               bool
	subscriptionID     string

	publisher publisher.Config

	users       users.Config
	procurement procurement.Config
	server      server.Config
}

func (c *config) RegisterFlags(f *flag.FlagSet) {
	flag.IntVar(&c.server.HTTPListenPort, "port", 80, "HTTP port for the Cloud Launcher's GCP Pub/Sub push webhook")
	flag.StringVar(&c.endpoint, "webhook-endpoint", "https://frontend.dev.weave.works/api/gcp-launcher/webhook?secret=FILLMEIN", "Endpoint this webhook is accessible from the outside")
	flag.BoolVar(&c.createSubscription, "pubsub-api.create-subscription", false, "Enable/Disable programmatic creation of the Pub/Sub subscription.")
	flag.BoolVar(&c.pull, "pubsub-api.pull", false, "(dev only!) Whether to use pull rather than push")
	flag.StringVar(&c.subscriptionID, "pubsub-api.subscription-id", "gcp-subscriptions", "Arbitrary name that denotes the Pub/Sub subscription 'pushing' to this webhook.")

	c.publisher.RegisterFlags(f)
	c.procurement.RegisterFlags(f)
	c.users.RegisterFlags(f)
	c.server.RegisterFlags(f)
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

	if err := logging.Setup(cfg.server.LogLevel.String()); err != nil {
		log.Fatalf("Error configuring logging: %v", err)
		return
	}
	cfg.server.Log = logging.Logrus(log.StandardLogger())

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

	cfg.server.MetricsNamespace = common.PrometheusNamespace
	server, err := server.New(cfg.server)
	if err != nil {
		log.Fatalf("Failed to start GCP Cloud Launcher webhook: %v", err)
	}
	defer server.Shutdown()
	server.HTTP.Handle("/", webhook.New(handler)).Methods("POST").Name("webhook")

	if cfg.pull {
		receiveSubscription(&cfg, forwardMessage(cfg.secret, cfg.server.HTTPListenPort))
	} else if cfg.createSubscription {
		createSubscription(&cfg)
	}

	log.Infof("Starting server")
	server.Run()
}

func createPublisher(cfg *config) *publisher.Publisher {
	log.Infof("Creating GCP Pub/Sub subscription [projects/%v/subscriptions/%v]...", cfg.publisher.ProjectID, cfg.subscriptionID)
	pub, err := publisher.New(context.Background(), cfg.publisher)
	if err != nil {
		log.Fatalf("Failed creating Pub/Sub publisher: %v", err)
	}
	return pub
}

func forwardMessage(secret string, port int) func(dto.Message) error {
	return func(msg dto.Message) error {
		ev := dto.Event{Subscription: "projects/foo/subscriptions/bar", Message: msg}
		bs, err := json.Marshal(ev)
		if err != nil {
			return err
		}
		client := http.Client{
			Timeout: 30 * time.Second,
		}
		u := url.Values{"secret": []string{secret}}
		resp, err := client.Post(fmt.Sprintf("http://localhost:%d?%s", port, u.Encode()), "application/json", bytes.NewReader(bs))
		if err != nil {
			return err
		}
		resp.Body.Close()
		return err
	}
}

// createSubscription programmatically creates a GCP Pub/Sub subscription, signaling GCP Pub/Sub to "push" to our webhook.
// IMPORTANT:
// With more than one replica of this service, we might run into race conditions when creating the subscription.
// If/when this happens, we may want to consider either manually setting the subscription up in the GCP portal, or using Terraform to do it.
func createSubscription(cfg *config) {
	pub := createPublisher(cfg)
	defer pub.Close()
	sub, err := pub.CreateSubscription(cfg.subscriptionID, cfg.Endpoint(), cfg.publisher.AckDeadline)
	if err != nil {
		log.Fatalf("Failed subscribing to Pub/Sub topic: %v", err)
	}
	log.Infof("Created subscription [%s], awaiting messages at: %v", sub, cfg.Endpoint())
}

func receiveSubscription(cfg *config, callback func(msg dto.Message) error) {
	go func() {
		pub := createPublisher(cfg)
		defer pub.Close()
		err := pub.ReceiveSubscription(cfg.subscriptionID, cfg.createSubscription, cfg.publisher.AckDeadline, callback)
		if err != nil {
			log.Fatalf(err.Error())
		}
	}()
	log.Infof("Awaiting messages")
}
