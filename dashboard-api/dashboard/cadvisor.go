package dashboard

// This dashboard displays metrics "by pod", all line graphs, a line per pod.
var cadvisorDashboard = Dashboard{
	ID:   "cadvisor-resources",
	Name: "Resources",
	Sections: []Section{{
		Name: "CPU usage",
		Rows: []Row{{
			Panels: []Panel{{
				Type:  PanelLine,
				Unit:  Unit{Format: UnitNumeric, Explanation: "CPU seconds / second"},
				Query: `sum (rate(container_cpu_usage_seconds_total{image!='',namespace='{{namespace}}',_weave_pod_name='{{workload}}'}[{{range}}])) by (pod_name)`,
			}},
		}},
	}, {
		Name: "Memory usage",
		Rows: []Row{{
			Panels: []Panel{{
				Type:  PanelLine,
				Unit:  Unit{Format: UnitBytes},
				Query: `sum (container_memory_working_set_bytes{image!='',namespace='{{namespace}}',_weave_pod_name='{{workload}}'}) by (pod_name)`,
			}},
		}},
	}, {
		Name: "Constraints",
		Rows: []Row{{
			Panels: []Panel{{
				Title:    "CPU Throttling",
				Type:     PanelLine,
				Optional: true,
				Unit:     Unit{Format: UnitPercent, Explanation: "Percentage of scheduling periods throttled"},
				Query: `sum(increase(container_cpu_cfs_throttled_periods_total{image!='',namespace='{{namespace}}',_weave_pod_name='{{workload}}'}[1m])) by (pod)` +
					`/` +
					`sum(increase(container_cpu_cfs_periods_total{image!='',namespace='{{namespace}}',_weave_pod_name='{{workload}}'}[1m])) by (pod)`,
			}, {
				Title:    "Memory Paging",
				Type:     PanelLine,
				Optional: true,
				Unit:     Unit{Format: UnitNumeric, Explanation: "Page faults / second"},
				Query:    `sum (rate(container_memory_failures_total{scope='container',failure_type='pgmajfault',namespace='{{namespace}}',_weave_pod_name='{{workload}}'}[1m])) by (pod_name)`,
			}},
		}},
	}, {
		Name: "GPU",
		Rows: []Row{{
			Panels: []Panel{{
				Title:    "GPU usage",
				Type:     PanelLine,
				Optional: true,
				Unit:     Unit{Format: UnitNumeric, Explanation: "GPU seconds / second"},
				// Already a rate: "Percent of time over the past sample period during which the accelerator was actively processing."
				// See also: https://github.com/google/cadvisor/blob/08f0c239/metrics/prometheus.go#L334-L335
				Query: `avg (container_accelerator_duty_cycle{image!='',namespace='{{namespace}}',_weave_pod_name='{{workload}}'}) by (pod_name)`,
			}, {
				Title:    "GPU memory usage",
				Type:     PanelLine,
				Optional: true,
				Unit:     Unit{Format: UnitBytes},
				Query:    `sum (container_accelerator_memory_used_bytes{image!='',namespace='{{namespace}}',_weave_pod_name='{{workload}}'}) by (pod_name)`,
			}},
		}},
	}, {
		Name: "Network traffic",
		Rows: []Row{{
			Panels: []Panel{{
				Title: "Incoming",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitBytes},
				Query: `sum (rate(container_network_receive_bytes_total{image!='',interface='eth0',namespace='{{namespace}}',_weave_pod_name='{{workload}}'}[{{range}}])) by (pod_name)`,
			}, {
				Title: "Outgoing",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitBytes},
				Query: `sum (rate(container_network_transmit_bytes_total{image!='',interface='eth0',namespace='{{namespace}}',_weave_pod_name='{{workload}}'}[{{range}}])) by (pod_name)`,
			}},
		}},
	}, {
		Name: "Disk I/O",
		Rows: []Row{{
			// container_fs_reads_bytes_total and container_fs_writes_bytes_total are somethings missing!
			// https://github.com/weaveworks/service/issues/1893
			Panels: []Panel{{
				Title:    "Bandwidth (read)",
				Type:     PanelLine,
				Optional: true,
				Unit:     Unit{Format: UnitBytes},
				Query:    `sum (rate(container_fs_reads_bytes_total{image!='',namespace='{{namespace}}',_weave_pod_name='{{workload}}'}[{{range}}])) by (pod_name)`,
			}, {
				Title:    "Bandwidth (write)",
				Type:     PanelLine,
				Optional: true,
				Unit:     Unit{Format: UnitBytes},
				Query:    `sum (rate(container_fs_writes_bytes_total{image!='',namespace='{{namespace}}',_weave_pod_name='{{workload}}'}[{{range}}])) by (pod_name)`,
			}},
		}, {
			Panels: []Panel{{
				Title: "Operations per second (read)",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitNumeric},
				Query: `sum (rate(container_fs_reads_total{image!='',namespace='{{namespace}}',_weave_pod_name='{{workload}}'}[{{range}}])) by (pod_name)`,
			}, {
				Title: "Operations per second (write)",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitNumeric},
				Query: `sum (rate(container_fs_writes_total{image!='',namespace='{{namespace}}',_weave_pod_name='{{workload}}'}[{{range}}])) by (pod_name)`,
			}},
		}},
	}},
}

var cadvisor = &promqlProvider{
	dashboard: &cadvisorDashboard,
}
