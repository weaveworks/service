package featureflag

// Enabler lets you check whether a feature flag is enabled or not.
// This interface allows you to encapsulate arbitrarily complex logic to determine whether a feature flag should be enabled or not.
type Enabler interface {
	IsEnabled() bool
}
