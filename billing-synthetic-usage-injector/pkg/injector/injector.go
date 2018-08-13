package injector

import (
	multierror "github.com/hashicorp/go-multierror"
	"github.com/weaveworks/service/billing-synthetic-usage-injector/pkg/injector/config"
	"github.com/weaveworks/service/billing-synthetic-usage-injector/pkg/injector/runnable"
	"github.com/weaveworks/service/billing-synthetic-usage-injector/pkg/injector/scope/probe"
)

// SyntheticUsageInjector is responsible for faking a whole Weave Cloud
// instance, in order to test our billing pipeline end-to-end.
// It wraps around injector.Runnable elements which do the actual usage
// reporting, and itself is an injector.Runnable, so effectively is a
// composite/decorator around these.
type SyntheticUsageInjector struct {
	runnables []runnable.Runnable
}

// Start reporting synthetic usage.
func (injector *SyntheticUsageInjector) Start() {
	for _, runnable := range injector.runnables {
		go runnable.Start()
	}
}

// Stop reporting synthetic usage.
func (injector *SyntheticUsageInjector) Stop() error {
	var errs error
	for _, runnable := range injector.runnables {
		if err := runnable.Stop(); err != nil {
			errs = multierror.Append(errs, err)
		}
	}
	return errs
}

// NewSyntheticUsageInjector instantiates a new SyntheticUsageInjector.
func NewSyntheticUsageInjector(config *config.Flags) (*SyntheticUsageInjector, error) {
	scopeProbe, err := probe.NewFakeScopeProbe(config)
	if err != nil {
		return nil, err
	}
	return &SyntheticUsageInjector{
		runnables: []runnable.Runnable{
			scopeProbe,
		},
	}, nil
}
