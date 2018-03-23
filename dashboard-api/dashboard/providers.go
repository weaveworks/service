package dashboard

import (
	"strings"

	"github.com/pkg/errors"
	"github.com/prometheus/prometheus/promql"
)

type provider interface {
	Init() error
	GetRequiredMetrics() []string
	GetDashboards() []Dashboard
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
	dashboard       Dashboard
}

func (p *staticProvider) Init() error {
	return nil
}

func (p *staticProvider) GetRequiredMetrics() []string {
	return p.requiredMetrics
}

func (p *staticProvider) GetDashboards() []Dashboard {
	return []Dashboard{p.dashboard}
}

type promqlProvider struct {
	requiredMetrics []string
	dashboard       Dashboard
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
	for i := range p.dashboard.Sections {
		section := &p.dashboard.Sections[i]
		for j := range section.Rows {
			row := &section.Rows[j]
			for k := range row.Panels {
				panel := &row.Panels[k]

				// Do the bare minimum to make the query parsable.
				query := replacer.Replace(panel.Query)

				metrics, err := parseMetrics(query)
				if err != nil {
					return errors.Wrap(err, query)
				}

				for _, metric := range metrics {
					metricsMap[metric] = true
				}
			}
		}
	}

	for metric := range metricsMap {
		p.requiredMetrics = append(p.requiredMetrics, metric)
	}

	return nil
}

func (p *promqlProvider) GetRequiredMetrics() []string {
	return p.requiredMetrics
}

func (p *promqlProvider) GetDashboards() []Dashboard {
	return []Dashboard{p.dashboard}
}
