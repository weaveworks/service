package main

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	requestLatency = prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Namespace: "scope",
		Subsystem: "users",
		Name:      "request_duration_nanoseconds",
		Help:      "Time spent serving HTTP requests.",
	}, []string{"method", "route", "status_code"})
)

func makePrometheusHandler() http.Handler {
	prometheus.MustRegister(requestLatency)
	return prometheus.Handler()
}
