package config_test

import (
	"flag"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/weaveworks/service/billing-synthetic-usage-injector/pkg/injector/config"
)

func TestParsingEmptyCLIOptionsAndArgumentsShouldReturnDefaults(t *testing.T) {
	config := config.Flags{}
	cli := flag.NewFlagSet("test", flag.ContinueOnError)
	config.RegisterFlags(cli)
	err := cli.Parse([]string{})
	assert.NoError(t, err)
	assert.Equal(t, uint(1), config.NumHosts)
	assert.NotNil(t, config.Scope)
	assert.Equal(t, "https://frontend.dev.weave.works.:443", config.Scope.URL)
	assert.Equal(t, "", config.Scope.Token)
	assert.Equal(t, 3*time.Second, config.Scope.PublishInterval)
	assert.Equal(t, 1*time.Second, config.Scope.SpyInterval)
	assert.False(t, config.Scope.NoControls)
	assert.False(t, config.Scope.Insecure)
}

func TestParsingCLIOptionsAndArgumentsShouldOverrideDefaults(t *testing.T) {
	config := config.Flags{}
	cli := flag.NewFlagSet("test", flag.ContinueOnError)
	config.RegisterFlags(cli)
	err := cli.Parse([]string{
		"-num-hosts=3",
		"-scope.url=https://cloud.weave.works.:443",
		"-scope.token=kedopsmaziijo955ppnbf3jhju1rszke",
		"-scope.publish.interval=10s",
		"-scope.spy.interval=3s",
		"-scope.no-controls=true",
		"-scope.insecure=true",
	})
	assert.NoError(t, err)
	assert.Equal(t, uint(3), config.NumHosts)
	assert.NotNil(t, config.Scope)
	assert.Equal(t, "https://cloud.weave.works.:443", config.Scope.URL)
	assert.Equal(t, "kedopsmaziijo955ppnbf3jhju1rszke", config.Scope.Token)
	assert.Equal(t, 10*time.Second, config.Scope.PublishInterval)
	assert.Equal(t, 3*time.Second, config.Scope.SpyInterval)
	assert.True(t, config.Scope.NoControls)
	assert.True(t, config.Scope.Insecure)
}
