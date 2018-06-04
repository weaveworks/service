package dashboard

import (
	"github.com/weaveworks/service/dashboard-api/aws"
)

var awsRDSDashboard = Dashboard{
	ID:   aws.RDS.Type.ToDashboardID(),
	Name: "RDS",
	Sections: []Section{{
		Name: "System",
		Rows: []Row{{
			Panels: []Panel{{
				Title: "CPU utilization",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitNumeric},
				Query: `sum(aws_rds_cpuutilization_average{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}',dbinstance_identifier='{{identifier}}'}) by (dbinstance_identifier)`,
			}, {
				Title: "Available RAM",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitBytes},
				Query: `sum(aws_rds_freeable_memory_average{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}',dbinstance_identifier='{{identifier}}'}) by (dbinstance_identifier)`,
			}},
		}},
	}, {
		Name: "Database",
		Rows: []Row{{
			Panels: []Panel{{
				Title: "Number of connections in use",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitNumeric},
				Query: `sum(aws_rds_database_connections_average{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}',dbinstance_identifier='{{identifier}}'}) by (dbinstance_identifier)`,
			}},
		}},
	}, {
		Name: "Disk",
		Rows: []Row{{
			Panels: []Panel{{
				Title: "Read IOPS",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitNumeric},
				Query: `sum(aws_rds_read_iops_average{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}',dbinstance_identifier='{{identifier}}'}) by (dbinstance_identifier)`,
			}, {
				Title: "Write IOPS",
				Type:  PanelLine,
				Unit:  Unit{Format: UnitNumeric},
				Query: `sum(aws_rds_write_iops_average{kubernetes_namespace='{{namespace}}',_weave_service='{{workload}}',dbinstance_identifier='{{identifier}}'}) by (dbinstance_identifier)`,
			}},
		}},
	}},
}

var awsRDS = &promqlProvider{
	dashboard: &awsRDSDashboard,
}
