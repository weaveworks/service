package dashboard

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	testDashboard = Dashboard{
		ID:   "test-dashboard",
		Name: "Test",
		Sections: []Section{{
			Name: "Thingy",
			Rows: []Row{{
				Panels: []Panel{{
					Type:  PanelLine,
					Query: `test_metric{_weave_service='{{workload}}'}`,
				}, {
					Type:     PanelLine,
					Optional: true,
					Query:    `test_optional_metric{_weave_service='{{workload}}'}`,
				}},
			}},
		}},
	}

	testProvider = &promqlProvider{
		dashboard: &testDashboard,
	}
)

func init() {
	testProvider.Init()
}

func TestContainsMetrics(t *testing.T) {
	tests := []struct {
		haystack map[string]bool
		needles  []string
		expected bool
	}{
		{map[string]bool{"foo": true, "bar": true}, []string{"foo"}, true},
		{map[string]bool{"foo": true, "bar": true}, []string{"foo", "bar"}, true},
		{map[string]bool{"foo": true, "bar": true}, []string{"foo", "baz"}, false},
	}

	for _, test := range tests {
		assert.Equal(t, test.expected, containsMetrics(test.haystack, test.needles))
	}
}

func TestGetDashboardForMetrics(t *testing.T) {
	tests := []struct {
		metrics            map[string]bool
		expectedDashboards []string
	}{
		{map[string]bool{}, nil},
		{map[string]bool{"test_metric": true}, []string{"test-dashboard"}},
	}

	for _, test := range tests {
		var gotDashboards []string

		dashboards := getDashboardsForMetrics([]provider{testProvider}, test.metrics)
		for i := range dashboards {
			gotDashboards = append(gotDashboards, dashboards[i].ID)
		}
		assert.Equal(t, test.expectedDashboards, gotDashboards)
	}

}

var testOptionalPanel = Dashboard{
	ID:   "test-optional-dashboard",
	Name: "Test Optional",
	Sections: []Section{{
		Name: "Section 1",
		Rows: []Row{{
			Panels: []Panel{{
				Type:  PanelLine,
				Query: `test_metric_0_0_0{_weave_service='{{workload}}'}`,
			}, {
				Type:  PanelLine,
				Query: `test_metric_0_0_1{_weave_service='{{workload}}'}`,
			}},
		}, {
			Panels: []Panel{{
				Type:  PanelLine,
				Query: `test_metric_0_1_0{_weave_service='{{workload}}'}`,
			}},
		}},
	}, {
		Name: "Section 2",
		Rows: []Row{{
			Panels: []Panel{{
				Type:  PanelLine,
				Query: `test_metric_1_0_0{_weave_service='{{workload}}'}`,
			}, {
				Type:  PanelLine,
				Query: `test_metric_1_0_1{_weave_service='{{workload}}'}`,
			}},
		}},
	}},
}

func getOptionalDashboard(t *testing.T, optionalPanels []Path) Dashboard {
	dashboard := testOptionalPanel.DeepCopy()

	for _, path := range optionalPanels {
		dashboard.Panel(&path).Optional = true
	}

	return dashboard
}

func TestDiscardOptionalPanels(t *testing.T) {
	type expected struct {
		numSections int
		numRows0    int
		numPanels00 int
		numPanels01 int
		numRows1    int
		numPanels10 int
	}
	tests := []struct {
		dashboard Dashboard
		expected  expected
	}{
		// Row with multiple panels, one of them being optional.
		{getOptionalDashboard(t, []Path{{0, 0, 1}}), expected{2, 2, 1, 1, 1, 2}},
		// Row with one optional panel, the row is discarded entirely.
		{getOptionalDashboard(t, []Path{{0, 1, 0}}), expected{2, 1, 2, -1, 1, 2}},
		// Section with only optional panels, the section is discarded entirely.
		{getOptionalDashboard(t, []Path{{1, 0, 0}, {1, 0, 1}}), expected{1, 2, 2, 1, -1, -1}},
	}

	for i := range tests {
		test := &tests[i]

		p := &promqlProvider{dashboard: &test.dashboard}
		p.Init()
		metrics := p.GetRequiredMetrics()
		metricsMap := make(map[string]bool)
		for _, m := range metrics {
			metricsMap[m] = true
		}
		got := discardOptionalPanels(p, metricsMap)
		assert.Equal(t, test.expected.numSections, len(got.Sections))
		assert.Equal(t, test.expected.numRows0, len(got.Sections[0].Rows))
		assert.Equal(t, test.expected.numPanels00, len(got.Sections[0].Rows[0].Panels))
		if test.expected.numPanels01 != -1 {
			assert.Equal(t, test.expected.numPanels01, len(got.Sections[0].Rows[1].Panels))
		}
		if test.expected.numRows1 != -1 {
			assert.Equal(t, test.expected.numRows1, len(got.Sections[1].Rows))
		}
		if test.expected.numPanels10 != -1 {
			assert.Equal(t, test.expected.numPanels10, len(got.Sections[1].Rows[0].Panels))
		}
	}
}

func TestResolveQueries(t *testing.T) {
	// work on a copy to not touch the original
	dashboard := testDashboard.DeepCopy()

	resolveQueries([]Dashboard{dashboard}, map[string]string{
		"workload": "bar",
	})
	assert.Equal(t, "test_metric{_weave_service='bar'}", dashboard.Sections[0].Rows[0].Panels[0].Query)
}

// getAllRequiredMetrics gets the union of the metrics required by a list of providers
func getAllRequiredMetrics(providers []provider) []string {
	metricsMap := make(map[string]bool)

	for _, provider := range providers {
		for _, metric := range provider.GetRequiredMetrics() {
			metricsMap[metric] = true
		}
	}

	metrics := make([]string, 0, len(metricsMap))
	for metric := range metricsMap {
		metrics = append(metrics, metric)
	}

	return metrics
}

func getAllDashboards() ([]Dashboard, error) {
	const ns = "default"
	const workload = "authfe"

	metrics := getAllRequiredMetrics(providers)
	return GetDashboards(metrics, map[string]string{
		"namespace":  ns,
		"workload":   workload,
		"identifier": "prod-users-vpc-database", // AWS dashboards
	})
}

// TestUniqueIDs ensures all dashboards we can produce have their own unique ID.
func TestUniqueIDs(t *testing.T) {
	err := Init()
	assert.NoError(t, err)

	dashboards, err := getAllDashboards()
	assert.NoError(t, err)

	ids := make(map[string]bool)
	for i := range dashboards {
		ids[dashboards[i].ID] = true
	}

	assert.Equal(t, len(dashboards), len(ids))

	Deinit()
}

// TestGolden ensures we don't deviate from a tested golden state. We don't have
// a way to test for the validity of the generated JSON (will the queries actual
// get any data?), but we can test what we produce now is the same as what we
// used to produce.
func TestGolden(t *testing.T) {
	err := Init()
	assert.NoError(t, err)

	dashboards, err := getAllDashboards()
	assert.NoError(t, err)

	for i := range dashboards {
		dashboard := &dashboards[i]

		golden := filepath.Join("testdata", fmt.Sprintf("%s-%s.golden", t.Name(), dashboard.ID))
		if *update {
			data, err := json.MarshalIndent(dashboard, "", "  ")
			assert.Nil(t, err)
			ioutil.WriteFile(golden, data, 0644)
		}

		expectedBytes, err := ioutil.ReadFile(golden)
		assert.Nil(t, err)
		gotBytes, err := json.MarshalIndent(dashboard, "", "  ")
		assert.NoError(t, err)
		assert.Nil(t, err)

		assert.Equal(t, string(expectedBytes), string(gotBytes))
	}

	Deinit()
}

func hasOptionalPanels(d *Dashboard) bool {
	optional := false
	forEachPanel(d, func(panel *Panel, path *Path) error {
		if panel.Optional {
			optional = true
			return errExitEarly
		}
		return nil
	})
	return optional
}

// getAllMetrics gets the union of the metrics required by a list of providers
// including the metrics for optional panels.
func getAllMetrics(providers []provider) []string {
	metricsMap := make(map[string]bool)

	for _, provider := range providers {
		p := provider.(*promqlProvider)
		for _, metrics := range p.pathToMetrics {
			for _, metric := range metrics {
				metricsMap[metric] = true
			}
		}
	}

	metrics := make([]string, 0, len(metricsMap))
	for metric := range metricsMap {
		metrics = append(metrics, metric)
	}

	return metrics
}

func getAllDashboardsWithOptionalPanels() ([]Dashboard, error) {
	const ns = "default"
	const workload = "authfe"

	metrics := getAllMetrics(providers)
	allDashboards, err := GetDashboards(metrics, map[string]string{
		"namespace":  ns,
		"workload":   workload,
		"identifier": "prod-users-vpc-database", // AWS dashboards
	})
	if err != nil {
		return nil, err
	}

	var dashboards []Dashboard
	for d := range allDashboards {
		dashboard := &allDashboards[d]
		if hasOptionalPanels(dashboard) {
			dashboards = append(dashboards, *dashboard)
		}
	}

	return dashboards, nil
}

// TestGolden only looks at required metrics. Here, we also produce golden files
// with all optional panels in.
func TestGoldenWithOptionalPanels(t *testing.T) {
	err := Init()
	assert.NoError(t, err)

	dashboards, err := getAllDashboardsWithOptionalPanels()
	assert.NoError(t, err)
	for i := range dashboards {
		dashboard := &dashboards[i]

		golden := filepath.Join("testdata", fmt.Sprintf("%s-%s-with-optional-panels.golden", t.Name(), dashboard.ID))
		if *update {
			data, err := json.MarshalIndent(dashboard, "", "  ")
			assert.Nil(t, err)
			ioutil.WriteFile(golden, data, 0644)
		}

		expectedBytes, err := ioutil.ReadFile(golden)
		assert.Nil(t, err)
		gotBytes, err := json.MarshalIndent(dashboard, "", "  ")
		assert.NoError(t, err)
		assert.Nil(t, err)

		assert.Equal(t, string(expectedBytes), string(gotBytes))
	}
	Deinit()
}
