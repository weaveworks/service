package dashboard

var memcachedDashboard = Dashboard{
	ID:   "memcached",
	Name: "memcached",
	Sections: []Section{{
		Name: "Cache",
		Rows: []Row{{
			Panels: []Panel{{
				Title: "Hit Rate",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitPercent},
				Query: `sum(rate(memcached_commands_total{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}',command=~'get|delete|incr|decr|cas|touch',status='hit'}[{{range}}])) / sum(rate(memcached_commands_total{kubernetes_namespace='{{namespace}}', _weave_service='{{workload}}',command=~'get|delete|incr|decr|cas|touch'}[{{range}}]))`,
			}},
		}, {
			Panels: []Panel{{
				Title: "Items in Cache",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitNumeric},
				Query: `sum(memcached_current_items{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}'})`,
			}, {
				Title: "Used Cache Memory",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitPercent},
				Query: `sum(memcached_current_bytes{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}'}) / sum(memcached_limit_bytes{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}'})`,
			}},
		}, {
			Panels: []Panel{{
				Title: "Evicted Items per Second",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitNumeric},
				Query: `sum(rate(memcached_items_evicted_total{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}'}[{{range}}]))`,
			}, {
				Title: "Reclaimed Items per Second",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitNumeric},
				Query: `sum(rate(memcached_items_reclaimed_total{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}'}[{{range}}]))`,
			}},
		}},
	}, {
		Name: "Commands",
		Rows: []Row{{
			Panels: []Panel{{
				Title: "Operations per Second",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitNumeric},
				Query: `sum(rate(memcached_commands_total{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}'}[{{range}}])) by (command)`,
			}, {
				Title: "Get/Set Ratio",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitNumeric},
				Query: `sum(rate(memcached_commands_total{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}',command='get'}[{{range}}])) / sum(rate(memcached_commands_total{kubernetes_namespace='{{namespace}}', _weave_service='{{workload}}',command='set'}[{{range}}]))`,
			}},
		}},
	}, {
		Name: "Network",
		Rows: []Row{{
			Panels: []Panel{{
				Title: "Bytes Read from Network per Second",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitBytes},
				Query: `sum(rate(memcached_read_bytes_total{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}'}[{{range}}])) by (kubernetes_pod_name)`,
			}, {
				Title: "Bytes Written to Network per Second",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitBytes},
				Query: `sum(rate(memcached_written_bytes_total{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}'}[{{range}}])) by (kubernetes_pod_name)`,
			}, {
				Title: "Number of Connections",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitNumeric},
				Query: `sum(memcached_current_connections{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}'}) by (kubernetes_pod_name)`,
			}},
		}},
	}},
}

var memcached = &staticProvider{
	requiredMetrics: []string{
		"memcached_commands_total",
		"memcached_items_evicted_total",
		"memcached_items_reclaimed_total",
		"memcached_current_items",
		"memcached_current_bytes",
		"memcached_limit_bytes",
		"memcached_read_bytes_total",
		"memcached_written_bytes_total",
		"memcached_current_connections",
	},
	dashboard: memcachedDashboard,
}
