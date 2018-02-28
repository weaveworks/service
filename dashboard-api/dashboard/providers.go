package dashboard

type provider interface {
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

func (p *staticProvider) GetRequiredMetrics() []string {
	return p.requiredMetrics
}

func (p *staticProvider) GetDashboards() []Dashboard {
	return []Dashboard{p.dashboard}
}
