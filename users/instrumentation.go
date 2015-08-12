package main

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	requestLatency = prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Namespace: "scope",
		Subsystem: "users",
		Name:      "request_latency_nanoseconds",
		Help:      "Time spent serving each HTTP request.",
	}, []string{"path", "status_code"})
)

func makePrometheusHandler() http.Handler {
	prometheus.MustRegister(requestLatency)
	return prometheus.Handler()
}
