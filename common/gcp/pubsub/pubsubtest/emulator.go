// +build integration

package pubsubtest

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/weaveworks/service/common/gcp/pubsub/publisher"
)

// Setup configures the emulator and creates a publisher.
func Setup(ctx context.Context, t *testing.T, cfg publisher.Config) publisher.Interface {
	os.Setenv("PUBSUB_EMULATOR_HOST", "127.0.0.1:8085")
	pub, err := publisher.New(ctx, cfg)
	assert.NoError(t, err)
	return pub
}
