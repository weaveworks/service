package dashboard

var goKitDashboard = Dashboard{
	ID:   "go-kit",
	Name: "Go-kit HTTP",
	Sections: []Section{{
		Name: "HTTP Request Rate and Latency",
		Rows: []Row{{
			Panels: []Panel{{
				Title: "Requests per second",
				Type:  PanelStackedArea,
				Unit:  Unit{Format: UnitNumeric},
				Query: `sum by (method)(rate(request_latency_microseconds_count{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}'}[{{range}}]))`,
			}, {
				Title: "Latency",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitSeconds},
				Query: `sum by (method)(rate(request_latency_microseconds_sum{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}'}[{{range}}])) * 1e-6 / sum by (method)(rate(request_latency_microseconds_count{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}'}[{{range}}]))`,
			}},
		}},
	}},
}

var goKit = &promqlProvider{
	dashboard: &goKitDashboard,
}
