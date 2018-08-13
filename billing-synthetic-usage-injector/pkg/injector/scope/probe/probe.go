package probe

import (
	"math/rand"
	"net"
	"net/url"
	"strconv"
	"time"

	"github.com/weaveworks/scope/common/xfer"
	"github.com/weaveworks/scope/probe"
	"github.com/weaveworks/scope/probe/appclient"
	"github.com/weaveworks/scope/probe/controls"

	"github.com/weaveworks/service/billing-synthetic-usage-injector/pkg/injector/config"
	"github.com/weaveworks/service/billing-synthetic-usage-injector/pkg/injector/runnable"
	"github.com/weaveworks/service/billing-synthetic-usage-injector/pkg/injector/scope/reporter"
)

// Usually, this is the current version available at https://checkpoint-api.weave.works/v1/check/scope-probe
const version = "billing-synthetic-usage-injector"

// fakeScopeProbe wraps around a real Scope probe, in order to better manage
// the lifecycle of its internal components, and propagate calls to Stop.
// It implements the runnable.Runnable interface.
type fakeScopeProbe struct {
	client   appclient.MultiAppClient
	resolver appclient.Resolver
	probe    *probe.Probe
}

// Start starts this fake's underlying Scope probe.
func (fake *fakeScopeProbe) Start() {
	fake.probe.Start()
}

// Stop stops this fake's underlying Scope probe and report.
func (fake *fakeScopeProbe) Stop() error {
	defer fake.client.Stop()
	defer fake.resolver.Stop()
	return fake.probe.Stop()
}

// NewFakeScopeProbe instantiates a new *probe.Probe which reports fake data to Scope.
func NewFakeScopeProbe(config *config.Flags) (runnable.Runnable, error) {
	clientFactory := func(hostname string, url url.URL) (appclient.AppClient, error) {
		token := config.Scope.Token
		if url.User != nil {
			token = url.User.Username()
			url.User = nil // erase credentials, as we use a special header
		}
		probeConfig := appclient.ProbeConfig{
			Token:        token,
			ProbeVersion: version,
			ProbeID:      probeID(),
			Insecure:     config.Scope.Insecure,
		}
		return appclient.NewAppClient(
			probeConfig, hostname, url,
			xfer.ControlHandlerFunc(controls.NewDefaultHandlerRegistry().HandleControlRequest),
		)
	}

	multiClient := appclient.NewMultiAppClient(clientFactory, config.Scope.NoControls)

	resolver, err := resolver(config, multiClient)
	if err != nil {
		return nil, err
	}

	probe := probe.New(config.Scope.SpyInterval, config.Scope.PublishInterval, multiClient, config.Scope.NoControls)

	fakeReporter := reporter.NewFakeScopeReporter(3)
	probe.AddReporter(fakeReporter)

	return &fakeScopeProbe{
		client:   multiClient,
		resolver: resolver,
		probe:    probe,
	}, nil
}

func probeID() string {
	rand.Seed(time.Now().UnixNano())
	return strconv.FormatInt(rand.Int63(), 16)
}

func resolver(config *config.Flags, multiClient appclient.MultiAppClient) (appclient.Resolver, error) {
	dnsLookupFn := net.LookupIP
	targets, err := appclient.ParseTargets([]string{config.Scope.URL})
	if err != nil {
		return nil, err
	}
	return appclient.NewResolver(appclient.ResolverConfig{
		Targets: targets,
		Lookup:  dnsLookupFn,
		Set:     multiClient.Set,
	})
}
