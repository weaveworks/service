package dashboard

import (
	"strings"

	"github.com/pkg/errors"
	"github.com/prometheus/prometheus/promql"
)

type provider interface {
	Init() error
	GetRequiredMetrics() []string
	GetDashboard() *Dashboard
}

var providers []provider

func registerProviders(p ...provider) {
	providers = append(providers, p...)
}

func unregisterAllProviders() {
	providers = nil
}

type staticProvider struct {
	requiredMetrics []string
	dashboard       *Dashboard
}

func (p *staticProvider) Init() error {
	return nil
}

func (p *staticProvider) GetRequiredMetrics() []string {
	return p.requiredMetrics
}

func (p *staticProvider) GetDashboard() *Dashboard {
	return p.dashboard
}

type promqlProvider struct {
	requiredMetrics []string
	dashboard       *Dashboard
}

// parseMetrics walks the expression AST looking for metric names. Only Vector
// and Matrix selectors have those.
func parseMetrics(query string) ([]string, error) {
	var metrics []string

	expr, err := promql.ParseExpr(query)
	if err != nil {
		return nil, err
	}

	promql.Inspect(expr, func(node promql.Node, path []promql.Node) bool {
		switch n := node.(type) {
		case *promql.VectorSelector:
			metrics = append(metrics, n.Name)
		case *promql.MatrixSelector:
			metrics = append(metrics, n.Name)
		}
		return true
	})

	return metrics, nil
}

func (p *promqlProvider) Init() error {
	replacer := strings.NewReplacer(
		"{{range}}", "2m",
	)

	// Collect the list of required metrics from the query themselves.
	metricsMap := make(map[string]bool)
	if err := forEachPanel(p.dashboard, func(panel *Panel, path *Path) error {
		// Do the bare minimum to make the query parsable.
		query := replacer.Replace(panel.Query)

		metrics, err := parseMetrics(query)
		if err != nil {
			return errors.Wrap(err, query)
		}

		for _, metric := range metrics {
			metricsMap[metric] = true
		}

		return nil
	}); err != nil {
		return err
	}

	for metric := range metricsMap {
		p.requiredMetrics = append(p.requiredMetrics, metric)
	}

	return nil
}

func (p *promqlProvider) GetRequiredMetrics() []string {
	return p.requiredMetrics
}

func (p *promqlProvider) GetDashboard() *Dashboard {
	return p.dashboard
}
