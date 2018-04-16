package dashboard

// This dashboard displays metrics "by pod", all line graphs, a line per pod.
var openfaasDashboard = Dashboard{
	ID:   "openfaas",
	Name: "OpenFaaS",
	Sections: []Section{{
		Name: "Traffic",
		Rows: []Row{{
			Panels: []Panel{{
				Title: "Function req/sec",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitNumeric},
				Query: `sum(rate(gateway_function_invocation_total{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}'}[{{range}}])) by (function_name)`,
			}},
		}},
	}, {
		Name: "RED Metrics",
		Rows: []Row{{
			Panels: []Panel{{
				Title:    "Execution duration",
				Type:     PanelLine,
				Optional: true,
				Unit:     Unit{Format: UnitSeconds},
				Query:    `(rate(gateway_functions_seconds_sum{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}'}[{{range}}])) / (rate(gateway_functions_seconds_count{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}'}[{{range}}]))`,
			}},
		}, {
			Panels: []Panel{{
				Title: "Successful req/sec",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitNumeric},
				Query: `rate(gateway_function_invocation_total{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}',code='200'}[{{range}}])`,
			}, {
				Title: "Failed req/sec",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitNumeric},
				Query: `rate(gateway_function_invocation_total{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}',code!='200'}[{{range}}])`,
			}},
		}, {
			Panels: []Panel{{
				Title: "Replicas per function",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitNumeric},
				Query: `sum(gateway_service_count{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}'}) by (function_name)`,
			}},
		}},
	}},
}

var openfaas = &promqlProvider{
	dashboard: &openfaasDashboard,
}
