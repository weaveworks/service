package dashboard

import (
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
