package featureflag

import (
	"fmt"
	"strings"
)

// GetFeatureFlagValue Parses a featureflag of the form foo:bar and returns bar
func GetFeatureFlagValue(flagID string, haystack []string) (string, bool) {
	prefix := fmt.Sprintf("%s:", flagID)
	for _, has := range haystack {
		if flagID == has {
			return "", true
		} else if strings.HasPrefix(has, prefix) {
			return strings.TrimPrefix(has, prefix), true
		}
	}
	return "", false
}

// HasFeatureAllFlags checks if a set of featureflags contains all the requested feature flags
func HasFeatureAllFlags(needles, haystack []string) bool {
	for _, f := range needles {
		found := false
		prefix := fmt.Sprintf("%s:", f)
		for _, has := range haystack {
			if f == has || strings.HasPrefix(has, prefix) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
