package main

import (
	"flag"
	"fmt"
	"net/http"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/tylerb/graceful"

	"github.com/weaveworks/service/common/logging"
	"github.com/weaveworks/service/configs/api"
)

func main() {
	var (
		logLevel    = flag.String("log.level", "info", "Logging level to use: debug | info | warn | error")
		port        = flag.Int("port", 80, "port to listen on")
		stopTimeout = flag.Duration("stop.timeout", 5*time.Second, "How long to wait for remaining requests to finish during shutdown")
	)
	flag.Parse()
	if err := logging.Setup(*logLevel); err != nil {
		logrus.Fatalf("Error configuring logging: %v", err)
		return
	}

	logrus.Debug("Debug logging enabled")
	logrus.Infof("Listening on port %d", *port)
	mux := http.NewServeMux()
	mux.Handle("/", api.New())
	mux.Handle("/metrics", prometheus.Handler())
	if err := graceful.RunWithErr(fmt.Sprintf(":%d", *port), *stopTimeout, mux); err != nil {
		logrus.Fatal(err)
	}
	logrus.Info("Gracefully shut down")
}
