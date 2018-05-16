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
				Query: `sum(avg_over_time(jvm_threads_current{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}'}[{{range}}])) by (kubernetes_pod_name)`,
			}, {
				Title: "Threads created per second",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitNumeric},
				Query: `sum(rate(jvm_threads_started_total{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}'}[{{range}}])) by (kubernetes_pod_name)`,
			}},
		}},
	}, {
		Name: "Memory",
		Rows: []Row{{
			Panels: []Panel{{
				Title: "Used memory",
				Help:  "Used memory is working set (~live objects in the heap) + garbage",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitBytes},
				Query: `sum(avg_over_time(jvm_memory_bytes_used{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}'}[{{range}}])) by (kubernetes_pod_name, area)`,
			}, {
				Title: "Memory used per pool",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitBytes},
				Query: `sum(avg_over_time(jvm_memory_pool_bytes_used{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}'}[{{range}}])) by (kubernetes_pod_name, pool)`,
			}},
		}},
	}, {
		Name: "Garbage collector",
		Rows: []Row{{
			Panels: []Panel{{
				Title: "Time spent in GC per second",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitSeconds},
				Query: `sum(rate(jvm_gc_collection_seconds_sum{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}'}[{{range}}])) by (kubernetes_pod_name, gc)`,
			}, {
				Title: "Number of GC cycles per second",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitNumeric},
				Query: `sum(rate(jvm_gc_collection_seconds_count{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}'}[{{range}}])) by (kubernetes_pod_name, gc)`,
			}},
		}},
	}},
}

var jvm = &promqlProvider{
	dashboard: &jvmDashboard,
}
