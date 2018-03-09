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
	PanelStackedArea           = "stacked-area"
	PanelStackedLine           = "stacked-line"
)

// Panel is a display of some data on a row, backed by a Prometheus query/
type Panel struct {
	Title string    `json:"title"`
	Type  PanelType `json:"type"`
	Query string    `json:"query"`
}

// Row is a line on a dashboard and holds one or more graphs.
type Row struct {
	Panels []Panel `json:"panels"`
}

// Section is a collection of rows. It can be used to group several graphs into
// a logical bundle.
type Section struct {
	Name string `json:"name"`
	Rows []Row  `json:"rows"`
}

// Dashboard is a collection of graphs categorized by sections and rows.
type Dashboard struct {
	// An ID uniquely identifying the dashboard type, eg cadvisor-system-resources.
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Sections []Section `json:"sections"`
}

func getDashboardsForMetrics(metrics []string) []Dashboard {
	var dashboards []Dashboard

	// For O(1) metric existence check.
	metricsMap := make(map[string]bool)
	for _, metric := range metrics {
		metricsMap[metric] = true
	}

	// Retrieve the list of dashboards.
	for _, provider := range providers {
		required := provider.GetRequiredMetrics()
		for _, req := range required {
			if _, ok := metricsMap[req]; !ok {
				continue
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
	templates := getDashboardsForMetrics(metrics)

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
		goRuntime,
	)
}

// Deinit reverses what Init does.
func Deinit() {
	unregisterAllProviders()
}
