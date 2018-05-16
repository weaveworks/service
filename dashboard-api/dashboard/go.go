package dashboard

// This dashboard displays metrics "by pod", all line graphs, a line per pod.
var goRuntimeDashboard = Dashboard{
	ID:   "go-runtime",
	Name: "Go",
	Sections: []Section{{
		Name: "Concurrency",
		Rows: []Row{{
			Panels: []Panel{{
				Title: "Number of goroutines",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitNumeric},
				Query: `sum(go_goroutines{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}'}) by (kubernetes_pod_name)`,
			}},
		}},
	}, {
		Name: "Memory",
		Rows: []Row{{
			Panels: []Panel{{
				Title: "Heap size",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitBytes},
				Query: `sum(avg_over_time(go_memstats_heap_alloc_bytes{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}'}[{{range}}])) by (kubernetes_pod_name)`,
			}, {
				Title: "Number of heap objects",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitNumeric},
				Query: `sum(avg_over_time(go_memstats_heap_objects{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}'}[{{range}}])) by (kubernetes_pod_name)`,
			}},
		}, {
			Panels: []Panel{{
				Title: "Number of heap objects allocated per second",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitNumeric},
				Query: `sum(rate(go_memstats_mallocs_total{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}'}[{{range}}])) by (kubernetes_pod_name)`,
			}, {
				Title: "Number of heap objects freed per second",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitNumeric},
				Query: `sum(rate(go_memstats_frees_total{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}'}[{{range}}])) by (kubernetes_pod_name)`,
			}},
		}},
	}, {
		Name: "Garbage collector",
		Rows: []Row{{
			Panels: []Panel{{
				Title: "Time spent in GC each second",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitSeconds},
				Query: `sum(rate(go_gc_duration_seconds_sum{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}'}[{{range}}])) by (kubernetes_pod_name)`,
			}},
		}, {
			Panels: []Panel{{
				Title: "Number of GC cycles per second",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitNumeric},
				Query: `sum(rate(go_gc_duration_seconds_count{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}'}[{{range}}])) by (kubernetes_pod_name)`,
			}, {
				Title: "Duration (75 percentile)",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitSeconds},
				Query: `max(go_gc_duration_seconds{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}',quantile='0.75'}) by (kubernetes_pod_name)`,
			}},
		}},
	}},
}

var goRuntime = &promqlProvider{
	dashboard: &goRuntimeDashboard,
}
