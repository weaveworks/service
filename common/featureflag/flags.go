package featureflag

// Billing feature flag enables billing for an organization
const Billing = "billing"

// NoBilling feature flag is used to mark instances for which we either:
// - bill externally
// - have deactivated billing.
// There is no logic behind it at the moment, this is just a convention,
// and useful to work around the fact we cannot filter instances without
// the "billing" flag, in the admin UI.
const NoBilling = "no-billing"
