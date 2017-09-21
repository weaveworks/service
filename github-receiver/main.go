package main

import (
	"flag"

	log "github.com/Sirupsen/logrus"
	"github.com/weaveworks/common/server"
)

func main() {
	var (
		fluxSvcURL    = flag.String("flux-svc-url", "fluxsvc.flux.svc.cluster.local.:80", "Flux service base URL")
		webhookSecret = flag.String("webhook-secret", "", "Github App webhook secret")
		cfg           = server.Config{
			MetricsNamespace: "github-receiver",
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
		"/webhook",
		makeHandler(*fluxSvcURL, []byte(*webhookSecret)),
	).Methods("POST").Name("receive_webhook")
	server.Run()
}
