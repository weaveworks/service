package dashboard

// This dashboard displays metrics "by pod", all line graphs, a line per pod.
var cadvisorDashboard = Dashboard{
	Name: "System Resources",
	Sections: []Section{{
		Name: "CPU",
		Rows: []Row{{
			Panels: []Panel{{
				Title: "CPU Usage",
				Type:  PanelLine,
				Query: `sum (rate(container_cpu_usage_seconds_total{image!='',namespace='{{namespace}}',_weave_pod_name='{{workload}}'}[{{range}}])) by (pod_name)`,
			}},
		}},
	}, {
		Name: "Memory",
		Rows: []Row{{
			Panels: []Panel{{
				Title: "Memory Usage",
				Type:  PanelLine,
				Query: `sum (rate(container_memory_working_set_bytes{image!='',namespace='{{namespace}}',_weave_pod_name='{{workload}}'}[{{range}}])) by (pod_name)`,
			}},
		}},
	}, {
		Name: "Network",
		Rows: []Row{{
			Panels: []Panel{{
				Title: "Incoming Network Traffic",
				Type:  PanelLine,
				Query: `sum (rate(container_network_receive_bytes_total{image!='',namespace='{{namespace}}',_weave_pod_name='{{workload}}'}[{{range}}])) by (pod_name)`,
			}, {
				Title: "Outgoing Network Traffic",
				Type:  PanelLine,
				Query: `sum (rate(container_network_transmit_bytes_total{image!='',namespace='{{namespace}}',_weave_pod_name='{{workload}}'}[{{range}}])) by (pod_name)`,
			}},
		}},
	}, {
		Name: "Disk",
		Rows: []Row{{
			Panels: []Panel{{
				Title: "I/O Bandwidth (Read)",
				Type:  PanelLine,
				Query: `sum (rate(container_fs_reads_bytes_total{image!='',namespace='{{namespace}}',_weave_pod_name='{{workload}}'}[{{range}}])) by (pod_name)`,
			}, {
				Title: "I/O Bandwidth (Write)",
				Type:  PanelLine,
				Query: `sum (rate(container_fs_writes_bytes_total{image!='',namespace='{{namespace}}',_weave_pod_name='{{workload}}'}[{{range}}])) by (pod_name)`,
			}},
		}, {
			Panels: []Panel{{
				Title: "I/O Operations per Second (Read)",
				Type:  PanelLine,
				Query: `sum (rate(container_fs_reads_total{image!='',namespace='{{namespace}}',_weave_pod_name='{{workload}}'}[{{range}}])) by (pod_name)`,
			}, {
				Title: "I/O Operations per Second (Write)",
				Type:  PanelLine,
				Query: `sum (rate(container_fs_writes_total{image!='',namespace='{{namespace}}',_weave_pod_name='{{workload}}'}[{{range}}])) by (pod_name)`,
			}},
		}},
	}},
}

var cadvisor = &staticProvider{
	requiredMetrics: []string{
		"container_cpu_usage_seconds_total",
		"container_memory_working_set_bytes",
		"container_network_receive_bytes_total",
		"container_network_transmit_bytes_total",
		"container_fs_reads_bytes_total",
		"container_fs_writes_bytes_total",
		"container_fs_reads_total",
		"container_fs_writes_total",
	},
	dashboard: cadvisorDashboard,
}