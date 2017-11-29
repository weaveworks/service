package server

import (
	"github.com/go-kit/kit/metrics/prometheus"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
)

var (
	// We export two differently named but otherwise identical metrics here
	// so that we can transition dashboards from one to the other without
	// being left without metrics at any point.

	connectedDaemonsSvc = prometheus.NewGaugeFrom(stdprometheus.GaugeOpts{
		Namespace: "flux",
		Subsystem: "fluxsvc",
		Name:      "connected_daemons_count",
		Help:      "Gauge of the current number of connected daemons",
	}, []string{})

	connectedDaemonsAPI = prometheus.NewGaugeFrom(stdprometheus.GaugeOpts{
		Namespace: "flux",
		Subsystem: "flux-api",
		Name:      "connected_daemons_count",
		Help:      "Gauge of the current number of connected daemons",
	}, []string{})
)
