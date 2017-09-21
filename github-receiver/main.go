package main

import (
	"flag"
	"log"
	"net/http"
)

func main() {
	var (
		fluxSvcUrl    = flag.String("flux-svc-url", "fluxsvc.flux.svc.cluster.local.:80", "Flux service base URL")
		webhookSecret = flag.String("webhook-secret", "", "Github App webhook secret")
	)
	flag.Parse()

	if *webhookSecret == "" {
		log.Fatal("webhook secret not set")
	}

	http.Handle("/webhook", makeHandler(*fluxSvcUrl, []byte(*webhookSecret)))
	http.ListenAndServe(":80", nil)
}
