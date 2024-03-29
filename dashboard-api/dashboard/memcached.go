package dashboard

var memcachedDashboard = Dashboard{
	ID:   "memcached",
	Name: "memcached",
	Sections: []Section{{
		Name: "Cache",
		Rows: []Row{{
			Panels: []Panel{{
				Title: "Hit rate",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitPercent},
				Query: `sum(rate(memcached_commands_total{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}',command=~'get|delete|incr|decr|cas|touch',status='hit'}[{{range}}])) / sum(rate(memcached_commands_total{kubernetes_namespace='{{namespace}}', _weave_service='{{workload}}',command=~'get|delete|incr|decr|cas|touch'}[{{range}}]))`,
			}},
		}, {
			Panels: []Panel{{
				Title: "Items in cache",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitNumeric},
				Query: `sum(memcached_current_items{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}'})`,
			}, {
				Title: "Used cache memory",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitPercent},
				Query: `sum(memcached_current_bytes{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}'}) / sum(memcached_limit_bytes{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}'})`,
			}},
		}, {
			Panels: []Panel{{
				Title: "Evicted items per second",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitNumeric},
				Query: `sum(rate(memcached_items_evicted_total{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}'}[{{range}}]))`,
			}, {
				Title: "Reclaimed items per second",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitNumeric},
				Query: `sum(rate(memcached_items_reclaimed_total{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}'}[{{range}}]))`,
			}},
		}},
	}, {
		Name: "Commands",
		Rows: []Row{{
			Panels: []Panel{{
				Title: "Operations per second",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitNumeric},
				Query: `sum(rate(memcached_commands_total{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}'}[{{range}}])) by (command)`,
			}, {
				Title: "Get/Set ratio",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitNumeric},
				Query: `sum(rate(memcached_commands_total{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}',command='get'}[{{range}}])) / sum(rate(memcached_commands_total{kubernetes_namespace='{{namespace}}', _weave_service='{{workload}}',command='set'}[{{range}}]))`,
			}},
		}},
	}, {
		Name: "Network",
		Rows: []Row{{
			Panels: []Panel{{
				Title: "Bytes read from network per second",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitBytes},
				Query: `sum(rate(memcached_read_bytes_total{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}'}[{{range}}])) by (kubernetes_pod_name)`,
			}, {
				Title: "Bytes written to network per second",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitBytes},
				Query: `sum(rate(memcached_written_bytes_total{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}'}[{{range}}])) by (kubernetes_pod_name)`,
			}, {
				Title: "Number of connections",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitNumeric},
				Query: `sum(memcached_current_connections{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}'}) by (kubernetes_pod_name)`,
			}},
		}},
	}},
}

var memcached = &promqlProvider{
	dashboard: &memcachedDashboard,
}
