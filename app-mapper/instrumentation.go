package main

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	apiReportRequestCount = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "scope",
		Subsystem: "appmapper",
		Name:      "api_report_request_count",
		Help:      "Total number of /api/report requests received.",
	})
	apiAppRequestCount = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "scope",
		Subsystem: "appmapper",
		Name:      "api_app_request_count",
		Help:      "Total number of /api/app requests received.",
	})
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
	getOrganizationsHostLatency = prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Namespace: "scope",
		Subsystem: "appmapper",
		Name:      "get_organizations_host_latency_nanoseconds",
		Help:      "Time spent in getOrganizationsHost.",
	}, []string{"error"})
	authenticateOrgLatency = prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Namespace: "scope",
		Subsystem: "appmapper",
		Name:      "authenticate_org_latency_nanoseconds",
		Help:      "Time spent in authenticateOrg.",
	}, []string{"error"})
	authenticateProbeLatency = prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Namespace: "scope",
		Subsystem: "appmapper",
		Name:      "authenticate_probe_latency_nanoseconds",
		Help:      "Time spent in authenticateProbe.",
	}, []string{"error"})
	fetchAppLatency = prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Namespace: "scope",
		Subsystem: "appmapper",
		Name:      "fetch_app_latency_nanoseconds",
		Help:      "Time spent in fetchApp.",
	}, []string{"error"})
	runAppLatency = prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Namespace: "scope",
		Subsystem: "appmapper",
		Name:      "run_app_latency_nanoseconds",
		Help:      "Time spent in runApp.",
	}, []string{"error"})
	destroyAppLatency = prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Namespace: "scope",
		Subsystem: "appmapper",
		Name:      "destroy_app_latency_nanoseconds",
		Help:      "Time spent in destroyApp.",
	}, []string{"error"})
	isAppRunningLatency = prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Namespace: "scope",
		Subsystem: "appmapper",
		Name:      "is_app_running_latency_nanoseconds",
		Help:      "Time spent in isAppRunning.",
	}, []string{"error"})
)

func makePrometheusHandler() http.Handler {
	prometheus.MustRegister(apiReportRequestCount)
	prometheus.MustRegister(apiAppRequestCount)
	prometheus.MustRegister(wsConnections)
	prometheus.MustRegister(wsRequestCount)
	prometheus.MustRegister(getOrganizationsHostLatency)
	prometheus.MustRegister(authenticateOrgLatency)
	prometheus.MustRegister(authenticateProbeLatency)
	prometheus.MustRegister(fetchAppLatency)
	prometheus.MustRegister(runAppLatency)
	prometheus.MustRegister(destroyAppLatency)
	prometheus.MustRegister(isAppRunningLatency)
	return prometheus.Handler()
}
