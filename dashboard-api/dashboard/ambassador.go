package dashboard

// This dashboard displays metrics "by pod", all line graphs, a line per pod.
var ambassadorDashboard = Dashboard{
	ID:   "ambassador",
	Name: "Ambassador",
	Sections: []Section{{
		Name: "Traffic",
		Rows: []Row{{
			Panels: []Panel{{
				Title: "Total connections per instance",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitNumeric},
				Query: `sum(envoy_server_total_connections{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}'}) by (kubernetes_pod_name)`,
			}},
		}},
	}, {
		Name: "RED metrics",
		Rows: []Row{{
			Panels: []Panel{{
				Title:    "Latency",
				Type:     PanelLine,
				Optional: true,
				Unit:     Unit{Format: UnitSeconds, Scale: 1e-3},
				Query:    `avg(envoy_listener_0_0_0_0_80_downstream_cx_length_ms{quantile="0.99"})`,
			}},
		}, {
			Panels: []Panel{{
				Title:    "2xx requests per second",
				Type:     PanelLine,
				Optional: true,
				Unit:     Unit{Format: UnitNumeric},
				Query:    `sum(rate(envoy_http_ingress_http_downstream_rq_2xx{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}'}[{{range}}])) by (kubernetes_pod_name)`,
			}, {
				Title:    "3xx requests per second",
				Type:     PanelLine,
				Optional: true,
				Unit:     Unit{Format: UnitNumeric},
				Query:    `sum(rate(envoy_http_ingress_http_downstream_rq_3xx{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}'}[{{range}}])) by (kubernetes_pod_name)`,
			}, {
				Title:    "4xx requests per second",
				Type:     PanelLine,
				Optional: true,
				Unit:     Unit{Format: UnitNumeric},
				Query:    `sum(rate(envoy_http_ingress_http_downstream_rq_4xx{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}'}[{{range}}])) by (kubernetes_pod_name)`,
			}, {
				Title:    "5xx requests per second",
				Type:     PanelLine,
				Optional: true,
				Unit:     Unit{Format: UnitNumeric},
				Query:    `sum(rate(envoy_http_ingress_http_downstream_rq_5xx{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}'}[{{range}}])) by (kubernetes_pod_name)`,
			}},
		}},
	}},
}

var ambassador = &promqlProvider{
	dashboard: &ambassadorDashboard,
}
