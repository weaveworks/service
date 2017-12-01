package main

import (
	"flag"

	log "github.com/sirupsen/logrus"
	"github.com/weaveworks/common/server"
	"github.com/weaveworks/service/common"
)

func main() {
	var (
		fluxURL       = flag.String("flux-url", "flux-api.flux.svc.cluster.local.:80", "Flux base URL")
		webhookSecret = flag.String("webhook-secret", "", "Github App webhook secret")
		cfg           = server.Config{
			MetricsNamespace:        common.PrometheusNamespace,
			RegisterInstrumentation: true,
		}
	)
	cfg.RegisterFlags(flag.CommandLine)
	flag.Parse()

	if *webhookSecret == "" {
		log.Fatal("webhook secret not set")
	}

	server, err := server.New(cfg)
	if err != nil {
		log.Fatal("error initialising server:", err)
	}
	defer server.Shutdown()

	server.HTTP.Handle(
		"/github-receiver/webhook",
		makeHandler(*fluxURL, []byte(*webhookSecret)),
	).Methods("POST").Name("receive_webhook")
	server.Run()
}
