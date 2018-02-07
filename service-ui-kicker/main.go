package main

import (
	"flag"
	"net/http"
	"os"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"gopkg.in/go-playground/webhooks.v3"
	"gopkg.in/go-playground/webhooks.v3/github"

	"github.com/weaveworks/service/service-ui-kicker/hookdispatcher"
	"github.com/weaveworks/service/service-ui-kicker/preview"
	"github.com/weaveworks/service/service-ui-kicker/scope"
)

const (
	secretEnv      = "WEBHOOK_SECRET"
	githubTokenEnv = "GITHUB_TOKEN"
)

var path = flag.String("path", "/webhooks", "webhook path for payload URL")
var port = flag.Int("port", 80, "webhook port for payload URL")

func init() {
	scope.Init()
}

func main() {
	flag.Parse()
	secret, ok := os.LookupEnv(secretEnv)
	if !ok {
		log.Fatalf("Webhook secret var %s not set\n", secretEnv)
	}
	githubToken, ok := os.LookupEnv(githubTokenEnv)
	if !ok {
		log.Errorf("github token var %s not set\n", githubTokenEnv)
	}
	hs := hookdispatcher.New()

	su := scope.NewUpdater()
	su.Start(hs)

	pl := preview.New(githubToken)
	pl.Start(hs)

	hook := github.New(&github.Config{Secret: secret})
	hook.RegisterEvents(hs.HandlePush, github.PushEvent)
	hook.RegisterEvents(hs.HandleStatus, github.StatusEvent)

	http.Handle("/metrics", promhttp.Handler())
	http.Handle(*path, prometheus.InstrumentHandler("webhook", webhooks.Handler(hook)))

	log.Info("Starting webhook server")
	log.Fatalf("ListenAndServe error: %s", http.ListenAndServe(":"+strconv.Itoa(*port), nil))
}
