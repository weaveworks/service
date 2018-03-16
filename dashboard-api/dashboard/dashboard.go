package dashboard

import (
	"errors"
	"strings"

	"github.com/mitchellh/copystructure"
)

// PanelType is the type of a panel.
type PanelType string

// The list of supported panel types.
const (
	PanelLine        PanelType = "line"
	PanelStackedArea PanelType = "stacked-area"
	PanelStackedLine PanelType = "stacked-line"
)

// UnitFormat specifies the values metric unit type.
type UnitFormat string

// The list of supported panel metric unit formats.
const (
	UnitNumeric UnitFormat = "numeric"
	UnitBytes   UnitFormat = "bytes"
	UnitPercent UnitFormat = "percent"
	UnitSeconds UnitFormat = "seconds"
)

// Unit describes the metric unit of graph values.
type Unit struct {
	Format      UnitFormat `json:"format"`
	Scale       float64    `json:"scale,omitempty"`
	Explanation string     `json:"explanation,omitempty"`
}

// Panel is a display of some data on a row, backed by a Prometheus query/
type Panel struct {
	Title string    `json:"title"`
	Help  string    `json:"help,omitempty"`
	Type  PanelType `json:"type"`
	Unit  Unit      `json:"unit"`
	Query string    `json:"query"`
}

// Row is a line on a dashboard and holds one or more graphs.
type Row struct {
	Panels []Panel `json:"panels"`
}

// Section is a collection of rows. It can be used to group several graphs into
// a logical bundle.
type Section struct {
	Name string `json:"name,omitempty"`
	Rows []Row  `json:"rows"`
}

// Dashboard is a collection of graphs categorized by sections and rows.
type Dashboard struct {
	// An ID uniquely identifying the dashboard type, eg cadvisor-system-resources.
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Sections []Section `json:"sections"`
}

func getDashboardsForMetrics(providers []provider, metrics []string) []Dashboard {
	var dashboards []Dashboard

	// For O(1) metric existence check.
	metricsMap := make(map[string]bool)
	for _, metric := range metrics {
		metricsMap[metric] = true
	}

	// Retrieve the list of dashboards.
nextProvider:
	for _, provider := range providers {
		required := provider.GetRequiredMetrics()
		for _, req := range required {
			if _, ok := metricsMap[req]; !ok {
				continue nextProvider
			}
		}

		dashboards = append(dashboards, provider.GetDashboards()...)
	}

	return dashboards
}

func resolveQueries(dashboards []Dashboard, oldnew ...string) {
	replacer := strings.NewReplacer(oldnew...)

	for d := range dashboards {
		dashboard := dashboards[d]
		for s := range dashboard.Sections {
			section := &dashboards[d].Sections[s]
			for r := range section.Rows {
				row := &section.Rows[r]
				for p := range row.Panels {
					panel := &row.Panels[p]
					panel.Query = replacer.Replace(panel.Query)
				}
			}
		}
	}
}

// GetServiceDashboards returns a list of dashboards that can be shown, given
// the list of metrics available for a service.
func GetServiceDashboards(metrics []string, namespace, workload string) ([]Dashboard, error) {
	templates := getDashboardsForMetrics(providers, metrics)
	if len(templates) == 0 {
		return nil, nil
	}

	// We deepcopy the dashboards as we're going to mutate the query
	copy, err := copystructure.Copy(templates)
	if err != nil {
		return nil, err
	}
	dashboards, ok := copy.([]Dashboard)
	if !ok {
		return nil, errors.New("couldn't deepcopy dashboards")
	}

	// resolve Queries fields
	resolveQueries(dashboards,
		"{{namespace}}", namespace,
		"{{workload}}", workload,
		"{{range}}", "2m",
	)

	return dashboards, nil
}

// Init initializes the dashboard package. It must be called first before any
// other API.
func Init() {
	registerProviders(
		cadvisor,
		memcached,
		goRuntime,
	)
}

// Deinit reverses what Init does.
func Deinit() {
	unregisterAllProviders()
}
