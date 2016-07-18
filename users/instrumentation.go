package main

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	requestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "scope",
		Name:      "request_duration_seconds",
		Help:      "Time spent (in seconds) serving HTTP requests.",
		Buckets:   prometheus.DefBuckets,
	}, []string{"method", "route", "status_code", "ws"})
	databaseRequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "scope",
		Name:      "database_request_duration_seconds",
		Help:      "Time spent (in seconds) doing database requests.",
		Buckets:   prometheus.DefBuckets,
	}, []string{"method", "status_code"})
)

func makePrometheusHandler() http.Handler {
	prometheus.MustRegister(requestDuration)
	prometheus.MustRegister(databaseRequestDuration)
	return prometheus.Handler()
}
