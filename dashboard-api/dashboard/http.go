package dashboard

var httpDashboard = Dashboard{
	ID:   "http",
	Name: "HTTP",
	Sections: []Section{{
		Name: "HTTP request rate and latency",
		Rows: []Row{{
			Panels: []Panel{{
				Title: "Requests per second",
				Type:  PanelStackedArea,
				Unit:  Unit{Format: UnitNumeric},
				Query: `sum by (status_code)(rate(http_request_duration_seconds_count{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}'}[{{range}}]))`,
			}, {
				Title: "Latency",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitSeconds},
				Query: `sum by (path)(rate(http_request_duration_seconds_sum{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}'}[{{range}}])) / sum by (path)(rate(http_request_duration_seconds_count{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}'}[{{range}}]))`,
			}},
		}},
	}},
}

var http = &promqlProvider{
	dashboard: &httpDashboard,
}
