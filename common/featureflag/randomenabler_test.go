package featureflag_test

import (
	"math"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/weaveworks/service/common/featureflag"
)

func TestPassingZeroDisablesFeatureFlagForEveryone(t *testing.T) {
	assert.False(t, featureflag.NewRandomEnablerWithCustomSource(0, ZerosSource{}).IsEnabled())
	assert.False(t, featureflag.NewRandomEnablerWithCustomSource(0, OnesSource{}).IsEnabled())
	assert.False(t, featureflag.NewRandomEnablerWithCustomSource(0, NinetyNineSource{}).IsEnabled())
	assert.False(t, featureflag.NewRandomEnabler(0).IsEnabled())
}

func TestEnablingFeatureOnePercentOfTheTime(t *testing.T) {
	assert.True(t, featureflag.NewRandomEnablerWithCustomSource(1, ZerosSource{}).IsEnabled())
	assert.False(t, featureflag.NewRandomEnablerWithCustomSource(1, OnesSource{}).IsEnabled())
	assert.False(t, featureflag.NewRandomEnablerWithCustomSource(1, NinetyNineSource{}).IsEnabled())
}

func TestPassingOneHundredEnablesFeatureFlagForEveryone(t *testing.T) {
	assert.True(t, featureflag.NewRandomEnablerWithCustomSource(100, ZerosSource{}).IsEnabled())
	assert.True(t, featureflag.NewRandomEnablerWithCustomSource(100, OnesSource{}).IsEnabled())
	assert.True(t, featureflag.NewRandomEnablerWithCustomSource(100, NinetyNineSource{}).IsEnabled())
	assert.True(t, featureflag.NewRandomEnabler(100).IsEnabled())
}

func TestPassingMoreThanOneHundredEnablesFeatureFlagForEveryone(t *testing.T) {
	assert.True(t, featureflag.NewRandomEnablerWithCustomSource(101, ZerosSource{}).IsEnabled())
	assert.True(t, featureflag.NewRandomEnablerWithCustomSource(101, OnesSource{}).IsEnabled())
	assert.True(t, featureflag.NewRandomEnablerWithCustomSource(101, NinetyNineSource{}).IsEnabled())
	assert.True(t, featureflag.NewRandomEnabler(101).IsEnabled())
	assert.True(t, featureflag.NewRandomEnablerWithCustomSource(math.MaxInt32, ZerosSource{}).IsEnabled())
	assert.True(t, featureflag.NewRandomEnablerWithCustomSource(math.MaxInt32, OnesSource{}).IsEnabled())
	assert.True(t, featureflag.NewRandomEnablerWithCustomSource(math.MaxInt32, NinetyNineSource{}).IsEnabled())
	assert.True(t, featureflag.NewRandomEnabler(math.MaxInt32).IsEnabled())
}

// ZerosSource always make Intn(100) return 0, which is useful for testing purposes here.
type ZerosSource struct {
}

func (s ZerosSource) Int63() int64 {
	return 0
}

func (s ZerosSource) Seed(seed int64) {
	// Ignore.
}

func TestZerosSource(t *testing.T) {
	assert.Equal(t, 0, rand.New(ZerosSource{}).Intn(100))
}

// OnesSource always make Intn(100) return 1, which is useful for testing purposes here.
type OnesSource struct {
}

func (s OnesSource) Int63() int64 {
	return 1 << 32
}

func (s OnesSource) Seed(seed int64) {
	// Ignore.
}

func TestOnesSource(t *testing.T) {
	assert.Equal(t, 1, rand.New(OnesSource{}).Intn(100))
}

// NinetyNineSource always make Intn(100) return 99, which is useful for testing purposes here.
type NinetyNineSource struct {
}

func (s NinetyNineSource) Int63() int64 {
	return 99 * (1 << 32)
}

func (s NinetyNineSource) Seed(seed int64) {
	// Ignore.
}
func TestNinetyNineSource(t *testing.T) {
	assert.Equal(t, 99, rand.New(NinetyNineSource{}).Intn(100))
}
