package dashboard

// This dashboard displays metrics "by pod", all line graphs, a line per pod.
var cadvisorDashboard = Dashboard{
	ID:   "cadvisor-resources",
	Name: "Resources",
	Sections: []Section{{
		Name: "CPU",
		Rows: []Row{{
			Panels: []Panel{{
				Title: "CPU Usage",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitNumeric, Explanation: "CPU seconds / second"},
				Query: `sum (rate(container_cpu_usage_seconds_total{image!='',namespace='{{namespace}}',_weave_pod_name='{{workload}}'}[{{range}}])) by (pod_name)`,
			}},
		}},
	}, {
		Name: "Memory",
		Rows: []Row{{
			Panels: []Panel{{
				Title: "Memory Usage",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitBytes},
				Query: `sum (container_memory_working_set_bytes{image!='',namespace='{{namespace}}',_weave_pod_name='{{workload}}'}) by (pod_name)`,
			}},
		}},
	}, {
		Name: "GPU",
		Rows: []Row{{
			Panels: []Panel{{
				Title: "GPU Usage",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitNumeric, Explanation: "GPU seconds / second"},
				// Already a rate: "Percent of time over the past sample period during which the accelerator was actively processing."
				// See also: https://github.com/google/cadvisor/blob/08f0c239/metrics/prometheus.go#L334-L335
				Query: `sum (container_accelerator_duty_cycle{image!='',namespace='{{namespace}}',_weave_pod_name='{{workload}}'}) by (pod_name)`,
			}, {
				Title: "GPU Memory Usage",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitBytes},
				Query: `sum (container_accelerator_memory_used_bytes{image!='',namespace='{{namespace}}',_weave_pod_name='{{workload}}'}) by (pod_name)`,
			}},
		}},
	}, {
		Name: "Network",
		Rows: []Row{{
			Panels: []Panel{{
				Title: "Incoming Network Traffic",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitBytes},
				Query: `sum (rate(container_network_receive_bytes_total{image!='',namespace='{{namespace}}',_weave_pod_name='{{workload}}'}[{{range}}])) by (pod_name)`,
			}, {
				Title: "Outgoing Network Traffic",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitBytes},
				Query: `sum (rate(container_network_transmit_bytes_total{image!='',namespace='{{namespace}}',_weave_pod_name='{{workload}}'}[{{range}}])) by (pod_name)`,
			}},
		}},
	}, {
		Name: "Disk",
		Rows: []Row{{
			// container_fs_reads_bytes_total and container_fs_writes_bytes_total are somethings missing!
			// https://github.com/weaveworks/service/issues/1893
			Panels: []Panel{{
				Title:    "I/O Bandwidth (Read)",
				Type:     PanelLine,
				Optional: true,
				Unit:     Unit{Format: UnitBytes},
				Query:    `sum (rate(container_fs_reads_bytes_total{image!='',namespace='{{namespace}}',_weave_pod_name='{{workload}}'}[{{range}}])) by (pod_name)`,
			}, {
				Title:    "I/O Bandwidth (Write)",
				Type:     PanelLine,
				Optional: true,
				Unit:     Unit{Format: UnitBytes},
				Query:    `sum (rate(container_fs_writes_bytes_total{image!='',namespace='{{namespace}}',_weave_pod_name='{{workload}}'}[{{range}}])) by (pod_name)`,
			}},
		}, {
			Panels: []Panel{{
				Title: "I/O Operations per Second (Read)",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitNumeric},
				Query: `sum (rate(container_fs_reads_total{image!='',namespace='{{namespace}}',_weave_pod_name='{{workload}}'}[{{range}}])) by (pod_name)`,
			}, {
				Title: "I/O Operations per Second (Write)",
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
