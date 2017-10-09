package featureflag

import (
	"math/rand"
	"time"

	log "github.com/sirupsen/logrus"
)

// RandomEnabler randomly enables a feature flag based on the provided percentage and source of randomness.
type RandomEnabler struct {
	percentage int
	rand       *rand.Rand
}

const max int = 100

// NewRandomEnabler creates a new RandomEnabler using a default source of randomness.
func NewRandomEnabler(percentage uint) *RandomEnabler {
	return NewRandomEnablerWithCustomSource(percentage, rand.NewSource(time.Now().UnixNano()))
}

// NewRandomEnablerWithCustomSource creates a new RandomEnabler using the provided source of randomness. This is mainly useful for testing.
func NewRandomEnablerWithCustomSource(percentage uint, source rand.Source) *RandomEnabler {
	if percentage > uint(max) {
		log.Warnf("Invalid percentage. Expected a value in the [0, 100] interval, but got: %v. Now overriding value to be 100 instead.", percentage)
		return &RandomEnabler{percentage: max, rand: rand.New(source)}
	}
	return &RandomEnabler{percentage: int(percentage), rand: rand.New(source)}
}

// IsEnabled returns true is the feature flag should be enabled for this occurrence, or false otherwise.
func (r RandomEnabler) IsEnabled() bool {
	return r.rand.Intn(max) < r.percentage
}
