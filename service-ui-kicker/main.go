package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"strconv"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/weaveworks/service/service-ui-kicker/committer"
	"github.com/weaveworks/service/service-ui-kicker/handler"
	"gopkg.in/go-playground/webhooks.v3"
	"gopkg.in/go-playground/webhooks.v3/github"
)

const (
	secretEnv = "WEBHOOK_SECRET"
	scopeRepo = "git@github.com:weaveworks/scope.git"
)

var path = flag.String("path", "/webhooks", "webhook path for payload URL")
var port = flag.Int("port", 80, "webhook port for payload URL")

var tasksCompleted = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "tasks_completed",
		Help: "Number of completed tasks for scope version update.",
	},
	[]string{},
)
var tasksFailed = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "tasks_failed",
		Help: "Number of failed tasks for scope version update.",
	},
	[]string{},
)
var completedDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
	Name: "completed_task_duration",
	Help: "Histogram for the completed tasks duration.",
},
)

func init() {
	prometheus.MustRegister(tasksCompleted, tasksFailed, completedDuration)
}

func main() {
	flag.Parse()
	secret, ok := os.LookupEnv(secretEnv)
	if !ok {
		log.Fatalf("Webhook secret %s not set\n", secretEnv)
	}
	hook := github.New(&github.Config{Secret: secret})
	hs := handler.New(scopeRepo)

	go func() {
		log.Info("Start waiting for new tasks...")
		for t := range hs.Tasks() {
			log.Infof("Got new task %s", t)
			begin := time.Now()
			if err := committer.PushUpdatedFile(context.Background(), t); err != nil {
				log.Errorf("Cannot clone, commit and push new version: %v", err)
				tasksFailed.With(prometheus.Labels{}).Inc()
			} else {
				log.Infof("Task done: version %q pushed to weaveworks/service-ui", t)
				tasksCompleted.With(prometheus.Labels{}).Inc()
				completedDuration.Observe(time.Since(begin).Seconds())
			}
		}
	}()

	hook.RegisterEvents(hs.HandlePush, github.PushEvent)
	hook.RegisterEvents(hs.HandleStatus, github.StatusEvent)

	http.Handle("/metrics", promhttp.Handler())
	http.Handle(*path, prometheus.InstrumentHandler("webhook", webhooks.Handler(hook)))

	log.Info("Starting webhook server")
	log.Fatalf("ListenAndServe error: %s", http.ListenAndServe(":"+strconv.Itoa(*port), nil))
}
