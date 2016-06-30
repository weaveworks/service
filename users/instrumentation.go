package main

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	requestDuration = prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Namespace: "scope",
		Name:      "request_duration_seconds",
		Help:      "Time spent (in seconds) serving HTTP requests.",
	}, []string{"method", "route", "status_code", "ws"})
)

func makePrometheusHandler() http.Handler {
	prometheus.MustRegister(requestDuration)
	return prometheus.Handler()
}
