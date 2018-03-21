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
	logLevel   string
	prometheus struct {
		uri     string
		timeout time.Duration
	}
	server server.Config
}

func (c *config) registerFlags(f *flag.FlagSet) {
	flag.StringVar(&c.logLevel, "log.level", "info", "The log level")
	flag.StringVar(&c.prometheus.uri, "prometheus.uri", "http://querier.cortex.svc.cluster.local", "Prometheus server URI")
	flag.DurationVar(&c.prometheus.timeout, "prometheus.timeout", 10*time.Second, "Timout when talking to the prometheus API")

	c.server.RegisterFlags(f)
}

func main() {
	cfg := &config{}
	cfg.registerFlags(flag.CommandLine)
	flag.Parse()
	cfg.server.MetricsNamespace = "service"

	if err := logging.Setup(cfg.logLevel); err != nil {
		log.Fatalf("error initializing logging: %v", err)
	}

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

	dashboard.Init()

	server.Run()
}
