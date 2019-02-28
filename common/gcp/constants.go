package gcp

// OAuthScopeCloudBillingPartnerSubscriptionsRO is the Google OAuth scope required for the client in the partner package to work.
const OAuthScopeCloudBillingPartnerSubscriptionsRO = "https://www.googleapis.com/auth/cloud-billing-partner-subscriptions.readonly"

// MarketplaceTokenParam is the key name for the JWT passed in by
// the GCP Marketplace when redirecting a user to either subscribe
// or sign in.
const MarketplaceTokenParam = "x-gcp-marketplace-token"
