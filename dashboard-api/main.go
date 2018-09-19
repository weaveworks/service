package main

import (
	"flag"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/weaveworks/common/logging"
	"github.com/weaveworks/common/server"
	"github.com/weaveworks/service/dashboard-api/dashboard"
)

type config struct {
	prometheus struct {
		uri     string
		timeout time.Duration
	}
	server server.Config
}

func (c *config) registerFlags(f *flag.FlagSet) {
	flag.StringVar(&c.prometheus.uri, "prometheus.uri", "http://querier.cortex.svc.cluster.local", "Prometheus server URI")
	flag.DurationVar(&c.prometheus.timeout, "prometheus.timeout", 10*time.Second, "Timeout when talking to the prometheus API")

	c.server.RegisterFlags(f)
}

func main() {
	cfg := &config{}
	cfg.registerFlags(flag.CommandLine)
	flag.Parse()
	cfg.server.MetricsNamespace = "service"

	if err := logging.Setup(cfg.server.LogLevel.String()); err != nil {
		log.Fatalf("error initializing logging: %v", err)
	}
	cfg.server.Log = logging.Logrus(log.StandardLogger())

	server, err := server.New(cfg.server)
	if err != nil {
		log.Fatalf("error initializing server: %v", err)
	}
	defer server.Shutdown()

	api, err := newAPI(cfg)
	if err != nil {
		log.Fatalf("error initializing API: %v", err)
	}
	api.registerRoutes(server.HTTP)

	if err := dashboard.Init(); err != nil {
		log.Fatalf("error initializing dashboards: %v", err)
	}

	server.Run()
}
