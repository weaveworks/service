package filter

import (
	"fmt"
	"strings"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/weaveworks/service/users"
)

// Organization filters organizations.
type Organization interface {
	Filter
	// MatchesOrg checks whether an organization matches this filter.
	MatchesOrg(users.Organization) bool
}

// ZuoraAccount filters an organization based on whether or not there's a Zuora account.
type ZuoraAccount bool

// Where filters by Zuora account.
func (z ZuoraAccount) Where() squirrel.Sqlizer {
	if bool(z) {
		return squirrel.Expr("zuora_account_number IS NOT NULL")
	}
	return squirrel.Eq{"zuora_account_number": nil}
}

// MatchesOrg checks whether the organization matches this filter.
func (z ZuoraAccount) MatchesOrg(o users.Organization) bool {
	if bool(z) {
		return o.ZuoraAccountNumber != ""
	}
	return o.ZuoraAccountNumber == ""
}

// GCP filters an organization whether it has been created through GCP.
type GCP bool

// Where filters by GCP.
func (g GCP) Where() squirrel.Sqlizer {
	if bool(g) {
		return squirrel.NotEq{"gcp_subscription_id": nil}
	}
	return squirrel.Eq{"gcp_subscription_id": nil}
}

// GCPSubscription filters an organization based on whether it has a running GCP subscription or not.
type GCPSubscription bool

// Where filters by a running GCP subscription.
func (g GCPSubscription) Where() squirrel.Sqlizer {
	if bool(g) {
		return squirrel.Expr("gcp_subscriptions.active AND gcp_subscriptions.subscription_name <> ''")
	}
	return squirrel.Expr("gcp_subscriptions.active = false OR gcp_subscriptions.subscription_name = ''")
}

// MatchesOrg checks whether the organization matches this filter.
func (g GCPSubscription) MatchesOrg(o users.Organization) bool {
	has := o.GCP != nil && o.GCP.Active && o.GCP.SubscriptionName != ""
	if bool(g) {
		return has
	}
	return !has
}

// TrialExpiredBy filters for organizations whose trials had expired by a
// given date.
type TrialExpiredBy time.Time

// ExtendQuery extends a query to filter by trial expiry.
func (t TrialExpiredBy) Where() squirrel.Sqlizer {
	return squirrel.Lt{"organizations.trial_expires_at": time.Time(t)}
}

// MatchesOrg checks whether an organization matches this filter.
func (t TrialExpiredBy) MatchesOrg(o users.Organization) bool {
	return o.TrialExpiresAt.Before(time.Time(t))
}

// TrialActiveAt filters for organizations whose trials were active at given
// date.
type TrialActiveAt time.Time

// ExtendQuery extends a query to filter by trial expiry.
func (t TrialActiveAt) Where() squirrel.Sqlizer {
	return squirrel.Gt{"organizations.trial_expires_at": time.Time(t)}
}

// MatchesOrg checks whether an organization matches this filter.
func (t TrialActiveAt) MatchesOrg(o users.Organization) bool {
	return o.TrialExpiresAt.After(time.Time(t))
}

// HasFeatureFlag filters for organizations that has the given feature flag.
type HasFeatureFlag string

// ExtendQuery extends a query to filter by feature flag.
func (f HasFeatureFlag) Where() squirrel.Sqlizer {
	return squirrel.Expr("?=ANY(feature_flags)", string(f))
}

// MatchesOrg checks whether an organization matches this filter.
func (f HasFeatureFlag) MatchesOrg(o users.Organization) bool {
	return o.HasFeatureFlag(string(f))
}

// ID filters for organizations with exactly this ID.
type ID string

// ExtendQuery extends a query to filter by ID.
func (i ID) Where() squirrel.Sqlizer {
	return squirrel.Eq{"id": string(i)}
}

// MatchesOrg checks whether an organization matches this filter.
func (i ID) MatchesOrg(o users.Organization) bool {
	return o.ID == string(i)
}

// ExternalID filters for organizations with exactly this external ID.
type ExternalID string

// ExtendQuery extends a query to filter by ID.
func (e ExternalID) Where() squirrel.Sqlizer {
	return squirrel.Eq{"external_id": string(e)}
}

// MatchesOrg checks whether an organization matches this filter.
func (e ExternalID) MatchesOrg(o users.Organization) bool {
	return o.ExternalID == string(e)
}

// SearchName finds organizations that have names that contain a string.
type SearchName string

// ExtendQuery extends a query to filter by having names that match our search.
func (s SearchName) Where() squirrel.Sqlizer {
	return squirrel.Expr("lower(organizations.name) LIKE ?",
		fmt.Sprint("%", strings.ToLower(string(s)), "%"))
}

// MatchesOrg checks whether an organization matches this filter.
func (s SearchName) MatchesOrg(o users.Organization) bool {
	return strings.Contains(o.Name, string(s))
}

// ProbeToken filters for organizations with exactly this token.
type ProbeToken string

// ExtendQuery extends a query to filter by token.
func (t ProbeToken) Where() squirrel.Sqlizer {
	return squirrel.Expr("organizations.probe_token = ?", string(t))
}

// MatchesOrg checks whether an organization matches this filter.
func (t ProbeToken) MatchesOrg(o users.Organization) bool {
	return strings.Contains(o.ProbeToken, string(t))
}
