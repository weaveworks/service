package injector_test

import (
	"flag"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/weaveworks/service/billing-synthetic-usage-injector/pkg/injector"
	"github.com/weaveworks/service/billing-synthetic-usage-injector/pkg/injector/config"
)

func TestInjector(t *testing.T) {
	config := config.Flags{}
	cli := flag.NewFlagSet("service-test", flag.ContinueOnError)
	config.RegisterFlags(cli)
	err := cli.Parse([]string{
		"-num-hosts=3",
		"-scope.url=https://frontend.dev.weave.works.:443",
		"-scope.token=kedopsmaziijo955ppnbf3jhju1rszke",
	})
	assert.NoError(t, err)
	injector, err := injector.NewSyntheticUsageInjector(&config)
	assert.NoError(t, err)
	go injector.Start()
	time.Sleep(15 * time.Second) // Wait for a few reports to be sent.
	err = injector.Stop()
	assert.NoError(t, err)
	// Ideally, we should read from BigQuery and assert reports were ingested,
	// but that's probably a bit too involved and expensive (BigQuery = $$$) for some CI test.
}
