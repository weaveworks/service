package main

import (
	"flag"
	"log"

	"github.com/weaveworks/common/server"
	"github.com/weaveworks/service/common"
	"github.com/weaveworks/service/common/gcp/pubsub/webhook"
	"github.com/weaveworks/service/common/gcp/partner"
	"github.com/weaveworks/service/gcp-launcher-webhook/subscription"
	"github.com/weaveworks/service/common/users"
)

func main() {
	var usersConfig users.Config
	var partnerConfig partner.Config
	port := flag.Int("port", 80, "HTTP port for the Cloud Launcher's GCP Pub/Sub push webhook.")
	flag.Parse()

	cfg := server.Config{
		HTTPListenPort:          *port,
		MetricsNamespace:        common.PrometheusNamespace,
		RegisterInstrumentation: true,
	}
	users, err := users.NewClient(usersConfig)
	if err != nil {
		log.Fatalf("error initialising users client: %v", err)
	}

	partner, err := partner.NewClient(partnerConfig)
	if err != nil {
		log.Fatalf("Failed creating Google Partner Subscriptions API client: %v", err)
	}

	server, err := server.New(cfg)
	if err != nil {
		log.Fatal("Failed to start GCP Cloud Launcher webhook", err)
	}
	defer server.Shutdown()


	server.HTTP.Handle(
		"/",
		webhook.New(&subscription.MessageHandler{
			Partner: partner,
			Users: users,
		}),
	).Methods("POST").Name("webhook")
	server.Run()
}
