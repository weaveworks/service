package main

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	requestDuration = prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Namespace: "scope",
		Subsystem: "appmapper",
		Name:      "request_duration_nanoseconds",
		Help:      "Time spent serving HTTP requests.",
	}, []string{"method", "route", "status_code"})
	wsConnections = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "scope",
		Subsystem: "appmapper",
		Name:      "websocket_connection_count",
		Help:      "Number of currently active websocket connections.",
	})
	wsRequestCount = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "scope",
		Subsystem: "appmapper",
		Name:      "websocket_request_count",
		Help:      "Total number of websocket requests received.",
	})
)

func makePrometheusHandler() http.Handler {
	prometheus.MustRegister(requestDuration)
	prometheus.MustRegister(wsConnections)
	prometheus.MustRegister(wsRequestCount)
	return prometheus.Handler()
}
