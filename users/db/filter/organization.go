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

// ExtendQuery extends a query to filter by Zuora account.
func (z ZuoraAccount) ExtendQuery(b squirrel.SelectBuilder) squirrel.SelectBuilder {
	if bool(z) {
		return b.Where("zuora_account_number IS NOT NULL")
	}
	return b.Where(map[string]interface{}{"zuora_account_number": nil})
}

// MatchesOrg checks whether the organization matches this filter.
func (z ZuoraAccount) MatchesOrg(o users.Organization) bool {
	if bool(z) {
		return o.ZuoraAccountNumber != ""
	}
	return o.ZuoraAccountNumber == ""
}

// GCPSubscription filters an organization based on whether it has a GCP subscription or not
type GCPSubscription bool

// ExtendQuery extends a query to filter by GCP subscription existence.
func (g GCPSubscription) ExtendQuery(b squirrel.SelectBuilder) squirrel.SelectBuilder {
	if bool(g) {
		return b.Where("gcp_subscription_id IS NOT NULL")
	}
	return b.Where(map[string]interface{}{"gcp_subscription_id": nil})
}

// MatchesOrg checks whether the organization matches this filter.
func (g GCPSubscription) MatchesOrg(o users.Organization) bool {
	if bool(g) {
		return o.GCP != nil
	}
	return o.GCP == nil
}

// TrialExpiredBy filters for organizations whose trials had expired by a
// given date.
type TrialExpiredBy time.Time

// ExtendQuery extends a query to filter by trial expiry.
func (t TrialExpiredBy) ExtendQuery(b squirrel.SelectBuilder) squirrel.SelectBuilder {
	return b.Where(squirrel.Lt{"organizations.trial_expires_at": time.Time(t)})
}

// MatchesOrg checks whether an organization matches this filter.
func (t TrialExpiredBy) MatchesOrg(o users.Organization) bool {
	return o.TrialExpiresAt.Before(time.Time(t))
}

// TrialActiveAt filters for organizations whose trials were active at given
// date.
type TrialActiveAt time.Time

// ExtendQuery extends a query to filter by trial expiry.
func (t TrialActiveAt) ExtendQuery(b squirrel.SelectBuilder) squirrel.SelectBuilder {
	return b.Where(squirrel.Gt{"organizations.trial_expires_at": time.Time(t)})
}

// MatchesOrg checks whether an organization matches this filter.
func (t TrialActiveAt) MatchesOrg(o users.Organization) bool {
	return o.TrialExpiresAt.After(time.Time(t))
}

// HasFeatureFlag filters for organizations that has the given feature flag.
type HasFeatureFlag string

// ExtendQuery extends a query to filter by feature flag.
func (f HasFeatureFlag) ExtendQuery(b squirrel.SelectBuilder) squirrel.SelectBuilder {
	return b.Where("?=ANY(feature_flags)", string(f))
}

// MatchesOrg checks whether an organization matches this filter.
func (f HasFeatureFlag) MatchesOrg(o users.Organization) bool {
	return o.HasFeatureFlag(string(f))
}

// ID filters for organizations with exactly this ID.
type ID string

// ExtendQuery extends a query to filter by ID.
func (i ID) ExtendQuery(b squirrel.SelectBuilder) squirrel.SelectBuilder {
	return b.Where(squirrel.Eq{"id": string(i)})
}

// MatchesOrg checks whether an organization matches this filter.
func (i ID) MatchesOrg(o users.Organization) bool {
	return o.ID == string(i)
}

// ExternalID filters for organizations with exactly this external ID.
type ExternalID string

// ExtendQuery extends a query to filter by ID.
func (e ExternalID) ExtendQuery(b squirrel.SelectBuilder) squirrel.SelectBuilder {
	return b.Where(squirrel.Eq{"external_id": string(e)})
}

// MatchesOrg checks whether an organization matches this filter.
func (e ExternalID) MatchesOrg(o users.Organization) bool {
	return o.ExternalID == string(e)
}

// SearchName finds organizations that have names that contain a string.
type SearchName string

// ExtendQuery extends a query to filter by having names that match our search.
func (s SearchName) ExtendQuery(b squirrel.SelectBuilder) squirrel.SelectBuilder {
	return b.Where("lower(organizations.name) LIKE ?",
		fmt.Sprint("%", strings.ToLower(string(s)), "%"))
}

// MatchesOrg checks whether an organization matches this filter.
func (s SearchName) MatchesOrg(o users.Organization) bool {
	return strings.Contains(o.Name, string(s))
}

// ProbeToken filters for organizations with exactly this token.
type ProbeToken string

// ExtendQuery extends a query to filter by token.
func (t ProbeToken) ExtendQuery(b squirrel.SelectBuilder) squirrel.SelectBuilder {
	return b.Where("organizations.probe_token = ?", string(t))
}

// MatchesOrg checks whether an organization matches this filter.
func (t ProbeToken) MatchesOrg(o users.Organization) bool {
	return strings.Contains(o.ProbeToken, string(t))
}
