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
