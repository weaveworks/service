package dashboard

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

var testDashbard = Dashboard{
	Name: "Test",
	Sections: []Section{{
		Name: "Thingy",
		Rows: []Row{{
			Panels: []Panel{{
				Type:  PanelLine,
				Query: `{{foo}}`,
			}},
		}},
	}},
}

func TestResolveQueries(t *testing.T) {
	resolveQueries([]Dashboard{testDashbard}, "{{foo}}", "bar")
	assert.Equal(t, "bar", testDashbard.Sections[0].Rows[0].Panels[0].Query)
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
	return GetServiceDashboards(metrics, ns, workload)
}

// TestUniqueIDs ensures all dashboards we can produce have their own unique ID.
func TestUniqueIDs(t *testing.T) {
	Init()

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
	Init()

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
