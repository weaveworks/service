package dashboard

import (
	"github.com/weaveworks/service/dashboard-api/aws"
)

var awsRDSDashboard = Dashboard{
	ID:   aws.RDS.Type.ToDashboardID(),
	Name: aws.RDS.Name,
	Sections: []Section{{
		Name: "System",
		Rows: []Row{{
			Panels: []Panel{{
				Title: "CPU utilization",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitNumeric},
				Query: `sum(aws_rds_cpuutilization_average{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}',dbinstance_identifier=~'{{identifier}}'}) by (dbinstance_identifier)`,
			}, {
				Title: "Available RAM",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitBytes},
				Query: `sum(aws_rds_freeable_memory_average{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}',dbinstance_identifier=~'{{identifier}}'}) by (dbinstance_identifier)`,
			}},
		}},
	}, {
		Name: "Database",
		Rows: []Row{{
			Panels: []Panel{{
				Title: "Number of connections in use",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitNumeric},
				Query: `sum(aws_rds_database_connections_average{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}',dbinstance_identifier=~'{{identifier}}'}) by (dbinstance_identifier)`,
			}},
		}},
	}, {
		Name: "Disk",
		Rows: []Row{{
			Panels: []Panel{{
				Title: "Read IOPS",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitNumeric},
				Query: `sum(aws_rds_read_iops_average{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}',dbinstance_identifier=~'{{identifier}}'}) by (dbinstance_identifier)`,
			}, {
				Title: "Write IOPS",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitNumeric},
				Query: `sum(aws_rds_write_iops_average{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}',dbinstance_identifier=~'{{identifier}}'}) by (dbinstance_identifier)`,
			}},
		}},
	}},
}

var awsRDS = &promqlProvider{
	dashboard: &awsRDSDashboard,
}

var awsClassicELBDashboard = Dashboard{
	ID:   aws.ELB.Type.ToDashboardID(),
	Name: aws.ELB.Name,
	Sections: []Section{{
		Name: "Requests",
		Rows: []Row{{
			Panels: []Panel{{
				Title: "Number of requests / connections",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitNumeric},
				Query: `sum(aws_elb_request_count_sum{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}',load_balancer_name=~'{{identifier}}'}) by (load_balancer_name)`,
			}, {
				Title:    "Backend connection errors",
				Optional: true,
				Type:     PanelLine,
				Unit:     Unit{Format: UnitBytes},
				Query:    `sum(aws_elb_backend_connection_errors{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}',load_balancer_name=~'{{identifier}}'}) by (load_balancer_name)`,
			}},
		}},
	}, {
		Name: "Hosts",
		Rows: []Row{{
			Panels: []Panel{{
				Title: "Number of healthy hosts",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitNumeric},
				Query: `sum(aws_elb_healthy_host_count_average{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}',load_balancer_name=~'{{identifier}}'}) by (load_balancer_name)`,
			}, {
				Title: "Number of unhealthy hosts",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitNumeric},
				Query: `sum(aws_elb_un_healthy_host_count_average{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}',load_balancer_name=~'{{identifier}}'}) by (load_balancer_name)`,
			}},
		}},
	}},
}

var awsClassicELB = &promqlProvider{
	dashboard: &awsClassicELBDashboard,
}
