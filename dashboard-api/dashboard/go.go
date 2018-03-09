package dashboard

// This dashboard displays metrics "by pod", all line graphs, a line per pod.
var goRuntimeDashboard = Dashboard{
	ID:   "go-runtime",
	Name: "Go",
	Sections: []Section{{
		Name: "Concurrency",
		Rows: []Row{{
			Panels: []Panel{{
				Title: "Number of Goroutines",
				Type:  PanelLine,
				Query: `go_goroutines{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}'}`,
			}},
		}},
	}, {
		Name: "Memory",
		Rows: []Row{{
			Panels: []Panel{{
				Title: "Heap Size",
				Type:  PanelLine,
				Query: `go_memstats_heap_alloc_bytes{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}'}`,
			}, {
				Title: "Number of Heap Objects",
				Type:  PanelLine,
				Query: `go_memstats_heap_objects{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}'}`,
			}},
		}, {
			Panels: []Panel{{
				Title: "Number of Heap Objects Allocated per Second",
				Type:  PanelLine,
				Query: `rate(go_memstats_mallocs_total{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}'}[{{range}}])`,
			}, {
				Title: "Number of Heap Objects Freed per Second",
				Type:  PanelLine,
				Query: `rate(go_memstats_frees_total{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}'}[{{range}}])`,
			}},
		}},
	}, {
		Name: "Garbage Collector",
		Rows: []Row{{
			Panels: []Panel{{
				Title: "Time spent in GC each Second",
				Type:  PanelLine,
				Query: `irate(go_gc_duration_seconds_sum{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}'}[{{range}}])`,
			}},
		}, {
			Panels: []Panel{{
				Title: "Number of GC Cycles per Second",
				Type:  PanelLine,
				Query: `irate(go_gc_duration_seconds_count{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}'}[{{range}}])`,
			}, {
				Title: "Duration (75 percentile)",
				Type:  PanelLine,
				Query: `go_gc_duration_seconds{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}',quantile='0.75'}`,
			}},
		}},
	}},
}

var goRuntime = &staticProvider{
	requiredMetrics: []string{
		"go_goroutines",
		"go_memstats_heap_alloc_bytes",
		"go_memstats_heap_objects",
		"go_memstats_mallocs_total",
		"go_memstats_frees_total",
		"go_gc_duration_seconds_sum",
		"go_gc_duration_seconds_count",
		"go_gc_duration_seconds",
	},
	dashboard: goRuntimeDashboard,
}
