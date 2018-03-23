package dashboard

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseMetrics(t *testing.T) {
	tests := []struct {
		query   string
		valid   bool
		metrics []string
	}{
		// VectorSelector
		{"some_metric", true, []string{"some_metric"}},
		{"some_metric{label='bar'}", true, []string{"some_metric"}},
		// MatrixSelector
		{"rate(some_metric{label='foo'}[2m])", true, []string{"some_metric"}},
		// A few more complex expressions, we use promql.Inspect to walk the AST, so
		// no need to test it in depth here.
		{"round(some_metric, 5)", true, []string{"some_metric"}},
		{"method_code:http_errors:rate5m / ignoring(code) group_left method:http_requests:rate5m", true,
			[]string{"method_code:http_errors:rate5m", "method:http_requests:rate5m"}},
		// Invalid query
		{"round(some_metric, 5", false, []string{"some_metric"}},
	}

	for _, test := range tests {
		metrics, err := parseMetrics(test.query)
		if !test.valid {
			assert.NotNil(t, err)
			continue
		}
		assert.NoError(t, err)
		assert.Equal(t, test.metrics, metrics)
	}
}

func TestPromQLProvider(t *testing.T) {
	p := &promqlProvider{dashboard: &testDashboard}
	err := p.Init()
	assert.NoError(t, err)
	assert.Equal(t, []string{"test_metric"}, p.GetRequiredMetrics())
	assert.Equal(t, []string{"test_metric"}, p.GetPanelMetrics(&Path{0, 0, 0}))
	assert.Equal(t, []string{"test_optional_metric"}, p.GetPanelMetrics(&Path{0, 0, 1}))
	assert.Nil(t, p.GetPanelMetrics(&Path{12, 13, 14}))
}
