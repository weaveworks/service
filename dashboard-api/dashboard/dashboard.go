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
	Title    string    `json:"title"`
	Optional bool      `json:"optional,omitempty"`
	Help     string    `json:"help,omitempty"`
	Type     PanelType `json:"type"`
	Unit     Unit      `json:"unit"`
	Query    string    `json:"query"`
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

// Panel returns the panel corresponding to path.
func (d *Dashboard) Panel(path *Path) *Panel {
	return &d.Sections[path.section].Rows[path.row].Panels[path.panel]
}

var errExitEarly = errors.New("exit early")

// forEachSection executes f for each section in d. f can return an error at any
// time, the walk through the section is stopped and the error returned.
func forEachSection(d *Dashboard, f func(*Section, *Path) error) error {
	for s := range d.Sections {
		section := &d.Sections[s]
		if err := f(section, &Path{s, -1, -1}); err != nil {
			return err
		}
	}
	return nil
}

// forEachRow executes f for each row in d. f can return an error at any time,
// the walk through the row is stopped and the error returned.
func forEachRow(d *Dashboard, f func(*Row, *Path) error) error {
	for s := range d.Sections {
		section := &d.Sections[s]
		for r := range section.Rows {
			row := &section.Rows[r]
			if err := f(row, &Path{s, r, -1}); err != nil {
				return err
			}
		}
	}
	return nil
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

func containsMetrics(haystack map[string]bool, needles []string) bool {
	for _, needle := range needles {
		if !haystack[needle] {
			return false
		}
	}
	return true
}

func discardOptionalPanels(provider provider, metrics map[string]bool) *Dashboard {
	dashboard := provider.GetDashboard().DeepCopy()

	// Remove optional panels
	forEachRow(&dashboard, func(row *Row, path *Path) error {
		var newPanels []Panel
		for p := range row.Panels {
			panel := &row.Panels[p]
			panelPath := &Path{path.section, path.row, p}
			if panel.Optional && !containsMetrics(metrics, provider.GetPanelMetrics(panelPath)) {
				continue
			}

			newPanels = append(newPanels, *panel)
		}
		row.Panels = newPanels
		return nil
	})

	// Garbage collect empty rows.
	forEachSection(&dashboard, func(section *Section, path *Path) error {
		var newRows []Row
		for r := range section.Rows {
			row := &section.Rows[r]
			if len(row.Panels) == 0 {
				continue
			}
			newRows = append(newRows, *row)
		}
		section.Rows = newRows
		return nil
	})

	// Garbage collect empty sections.
	var newSections []Section
	for s := range dashboard.Sections {
		section := &dashboard.Sections[s]
		if len(section.Rows) == 0 {
			continue
		}
		newSections = append(newSections, *section)
	}
	dashboard.Sections = newSections

	// We assume a dashboard cannot be composed of only optional panels, ie has at
	// least one panel.

	return &dashboard
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

		// Handle Optional:true panels.
		dashboard := discardOptionalPanels(provider, metricsMap)

		dashboards = append(dashboards, *dashboard)
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

	dashboards := getDashboardsForMetrics(providers, metricsMap)
	if len(dashboards) == 0 {
		return nil, nil
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
		http,
		goKit,
		cadvisor,
		openfaas,
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
