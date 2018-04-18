package dashboard

// This dashboard displays metrics "by pod", all line graphs, a line per pod.
var openfaasDashboard = Dashboard{
	ID:   "openfaas",
	Name: "OpenFaaS",
	Sections: []Section{{
		Name: "Traffic",
		Rows: []Row{{
			Panels: []Panel{{
				Title: "Function Requests per Second",
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
				Query:    `sum(rate(gateway_functions_seconds_sum{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}'}[{{range}}]) / rate(gateway_functions_seconds_count{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}'}[{{range}}])) by (function_name)`,
			}},
		}, {
			Panels: []Panel{{
				Title: "Successful Requests per Second",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitNumeric},
				Query: `sum(rate(gateway_function_invocation_total{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}',code='200'}[{{range}}])) by (function_name)`,
			}, {
				Title: "Failed Requests per Second",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitNumeric},
				Query: `sum(rate(gateway_function_invocation_total{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}',code!='200'}[{{range}}])) by (function_name)`,
			}},
		}, {
			Panels: []Panel{{
				Title: "Replicas per Function",
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
