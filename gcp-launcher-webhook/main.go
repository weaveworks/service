package main

import (
	"flag"
	"log"

	"github.com/weaveworks/common/server"
	"github.com/weaveworks/service/common"
	"github.com/weaveworks/service/common/gcp/pubsub/webhook"
	"github.com/weaveworks/service/gcp-launcher-webhook/handler"
)

func main() {
	port := flag.Int("port", 80, "HTTP port for the Cloud Launcher's GCP Pub/Sub push webhook.")
	flag.Parse()

	cfg := server.Config{
		HTTPListenPort:          *port,
		MetricsNamespace:        common.PrometheusNamespace,
		RegisterInstrumentation: true,
	}

	server, err := server.New(cfg)
	if err != nil {
		log.Fatal("Failed to start GCP Cloud Launcher webhook", err)
	}
	defer server.Shutdown()

	server.HTTP.Handle(
		"/",
		webhook.New(&handler.EventHandler{
			ActivationRequestedHandler: handler.NoOpHandler{},
			PlanChangeHandler:          handler.NoOpHandler{},
			CancelledHandler:           handler.CancelledHandler{},
			SuspendedHandler:           handler.SuspendedHandler{},
			ReactivatedHandler:         handler.ReactivatedHandler{},
			DeletedHandler:             handler.DeletedHandler{},
			DefaultHandler:             handler.NoOpHandler{},
		}),
	).Methods("POST").Name("webhook")
	server.Run()
}
