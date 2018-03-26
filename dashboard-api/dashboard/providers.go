package dashboard

import (
	"strings"

	"github.com/pkg/errors"
	"github.com/prometheus/prometheus/promql"
)

type provider interface {
	Init() error
	GetPanelMetrics(path *Path) []string
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

func (p *staticProvider) GetPanelMetrics(path *Path) []string {
	panic("Not Implemented")
}

func (p *staticProvider) GetRequiredMetrics() []string {
	return p.requiredMetrics
}

func (p *staticProvider) GetDashboard() *Dashboard {
	return p.dashboard
}

type promqlProvider struct {
	requiredMetrics []string
	pathToMetrics   map[string][]string
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

	// Parse the Dashboard queries and derive:
	//   - For each panel, the list of the metrics used in the query
	//   - The list of required  metrics
	p.pathToMetrics = make(map[string][]string)
	requiredMetricsMap := make(map[string]bool)
	if err := forEachPanel(p.dashboard, func(panel *Panel, path *Path) error {
		// Do the bare minimum to make the query parsable.
		query := replacer.Replace(panel.Query)

		metrics, err := parseMetrics(query)
		if err != nil {
			return errors.Wrap(err, query)
		}

		p.pathToMetrics[path.String()] = metrics

		// Don't collect metrics for optional panels in the list of required metrics.
		if panel.Optional {
			return nil
		}

		for _, metric := range metrics {
			requiredMetricsMap[metric] = true
		}

		return nil
	}); err != nil {
		return err
	}

	for metric := range requiredMetricsMap {
		p.requiredMetrics = append(p.requiredMetrics, metric)
	}

	return nil
}

func (p *promqlProvider) GetPanelMetrics(path *Path) []string {
	if metrics, ok := p.pathToMetrics[path.String()]; ok {
		return metrics
	}
	return nil
}

func (p *promqlProvider) GetRequiredMetrics() []string {
	return p.requiredMetrics
}

func (p *promqlProvider) GetDashboard() *Dashboard {
	return p.dashboard
}
