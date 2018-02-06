package common

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/weaveworks/common/instrument"
)

const (
	// PrometheusNamespace for all metrics in 'service'
	PrometheusNamespace = "service"
)

var (
	// RequestDuration is our standard histogram vector.
	RequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: PrometheusNamespace,
		Name:      "request_duration_seconds",
		Help:      "Time (in seconds) spent serving HTTP requests.",
		Buckets:   instrument.DefBuckets,
	}, []string{"method", "route", "status_code", "ws"})

	// DatabaseRequestDuration is our standard database histogram vector.
	DatabaseRequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: PrometheusNamespace,
		Name:      "database_request_duration_seconds",
		Help:      "Time spent (in seconds) doing database requests.",
		Buckets:   instrument.DefBuckets,
	}, []string{"method", "status_code"})
)
