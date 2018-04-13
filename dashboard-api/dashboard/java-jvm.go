package dashboard

var jvmDashboard = Dashboard{
	ID:   "java-jvm",
	Name: "JVM",
	Sections: []Section{{
		Name: "Concurrency",
		Rows: []Row{{
			Panels: []Panel{{
				Title: "Threads",
				Help:  "Current number of live threads including both daemon and non-daemon threads",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitNumeric},
				Query: `avg_over_time(jvm_threads_current{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}'}[{{range}}])`,
			}, {
				Title: "Threads Created per Second",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitNumeric},
				Query: `rate(jvm_threads_started_total{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}'}[{{range}}])`,
			}},
		}},
	}, {
		Name: "Memory",
		Rows: []Row{{
			Panels: []Panel{{
				Title: "Used Memory",
				Help:  "Used memory is working set (~live objects in the heap) + garbage",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitBytes},
				Query: `avg_over_time(jvm_memory_bytes_used{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}'}[{{range}}])`,
			}, {
				Title: "Memory Used per Pool",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitBytes},
				Query: `avg_over_time(jvm_memory_pool_bytes_used{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}'}[{{range}}])`,
			}},
		}},
	}, {
		Name: "Garbage Collector",
		Rows: []Row{{
			Panels: []Panel{{
				Title: "Time Spent in GC every second",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitSeconds},
				Query: `rate(jvm_gc_collection_seconds_sum{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}'}[{{range}}])`,
			}, {
				Title: "Number of GC Cycles per Second",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitNumeric},
				Query: `rate(jvm_gc_collection_seconds_count{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}'}[{{range}}])`,
			}},
		}},
	}},
}

var jvm = &promqlProvider{
	dashboard: &jvmDashboard,
}
