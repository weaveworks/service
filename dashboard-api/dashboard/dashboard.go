package dashboard

import (
	"errors"
	"strings"

	"github.com/mitchellh/copystructure"
)

// Config holds the list of template variables that can be used in dashboard queries.
type Config struct {
	Namespace string
	Workload  string
	Range     string
}

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

// DeepCopy returns a deep copy of a dashboard.
func (d *Dashboard) DeepCopy() Dashboard {
	copy, _ := copystructure.Copy(*d)
	return copy.(Dashboard)
}

// forEachPanel executes f for each panel in d. f can return an error at any
// time, the walk through the panels is stopped and the error returned.
func forEachPanel(d *Dashboard, f func(*Panel, *Path) error) error {
	for s := range d.Sections {
		section := &d.Sections[s]
		for r := range section.Rows {
			row := &section.Rows[r]
			for p := range row.Panels {
				panel := &row.Panels[p]
				if err := f(panel, &Path{s, r, p}); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func getDashboardsForMetrics(providers []provider, metricsMap map[string]bool) []Dashboard {
	var dashboards []Dashboard

	// Retrieve the list of dashboards.
nextProvider:
	for _, provider := range providers {
		required := provider.GetRequiredMetrics()
		for _, req := range required {
			if _, ok := metricsMap[req]; !ok {
				continue nextProvider
			}
		}

		dashboards = append(dashboards, *provider.GetDashboard())
	}

	return dashboards
}

func resolveQueries(dashboards []Dashboard, config *Config) {
	replacer := strings.NewReplacer(
		"{{namespace}}", config.Namespace,
		"{{workload}}", config.Workload,
		"{{range}}", config.Range,
	)

	for d := range dashboards {
		dashboard := &dashboards[d]
		forEachPanel(dashboard, func(panel *Panel, path *Path) error {
			panel.Query = replacer.Replace(panel.Query)
			return nil
		})
	}
}

// GetDashboardByID retrieves a dashboard by ID
func GetDashboardByID(ID string, config *Config) *Dashboard {
	for _, provider := range providers {
		dashboard := provider.GetDashboard()
		if dashboard.ID == ID {
			results := make([]Dashboard, 1, 1)

			results[0] = *dashboard
			resolveQueries(results, config)
			return &results[0]
		}
	}

	return nil
}

// GetServiceDashboards returns a list of dashboards that can be shown, given
// the list of metrics available for a service.
func GetServiceDashboards(metrics []string, namespace, workload string) ([]Dashboard, error) {
	// For O(1) metric existence checks.
	metricsMap := make(map[string]bool)
	for _, metric := range metrics {
		metricsMap[metric] = true
	}

	templates := getDashboardsForMetrics(providers, metricsMap)
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
	resolveQueries(dashboards, &Config{
		Namespace: namespace,
		Workload:  workload,
		Range:     "2m",
	})

	return dashboards, nil
}

// Init initializes the dashboard package. It must be called first before any
// other API.
func Init() error {
	registerProviders(
		cadvisor,
		memcached,
		jvm,
		goRuntime,
	)

	for _, provider := range providers {
		if err := provider.Init(); err != nil {
			return err
		}
	}

	return nil
}

// Deinit reverses what Init does.
func Deinit() {
	unregisterAllProviders()
}
