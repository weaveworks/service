package filter

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/weaveworks/service/users"
)

// OrganizationFilter filters organizations.
type OrganizationFilter interface {
	// ExtendQuery extends a query to filter by something.
	ExtendQuery(squirrel.SelectBuilder) squirrel.SelectBuilder
	// Matches checks whether an organization matches this filter.
	Matches(users.Organization) bool
}

// ZuoraAccount filters an organization based on whether or not there's a Zuora account.
type ZuoraAccount struct {
	Has bool
}

// ExtendQuery extends a query to filter by Zuora account.
func (z ZuoraAccount) ExtendQuery(b squirrel.SelectBuilder) squirrel.SelectBuilder {
	if z.Has {
		return b.Where("zuora_account_number IS NOT NULL")
	}
	return b.Where(map[string]interface{}{"zuora_account_number": nil})
}

// Matches checks whether the organization matches this filter.
//
// Must be kept in sync with ExtendQuery.
func (z ZuoraAccount) Matches(o users.Organization) bool {
	if z.Has {
		return o.ZuoraAccountNumber != ""
	}
	return o.ZuoraAccountNumber == ""
}

// TrialExpiredBy filters for organizations whose trials had expired by a
// given date.
type TrialExpiredBy struct {
	// When: Has the trial expired by this time?
	When time.Time
}

// ExtendQuery extends a query to filter by trial expiry.
//
// Must be kept in sync with Matches.
func (t TrialExpiredBy) ExtendQuery(b squirrel.SelectBuilder) squirrel.SelectBuilder {
	return b.Where("organizations.trial_expires_at < ?", t.When)
}

// Matches checks whether an organization matches this filter.
//
// Must be kept in sync with ExtendQuery.
func (t TrialExpiredBy) Matches(o users.Organization) bool {
	return o.TrialExpiresAt.Before(t.When)
}

// TrialActiveAt filters for organizations whose trials were active at given
// date.
type TrialActiveAt struct {
	When time.Time
}

// ExtendQuery extends a query to filter by trial expiry.
//
// Must be kept in sync with Matches.
func (t TrialActiveAt) ExtendQuery(b squirrel.SelectBuilder) squirrel.SelectBuilder {
	return b.Where("organizations.trial_expires_at > ?", t.When)
}

// Matches checks whether an organization matches this filter.
//
// Must be kept in sync with ExtendQuery.
func (t TrialActiveAt) Matches(o users.Organization) bool {
	return o.TrialExpiresAt.After(t.When)
}

// And combines many filters.
func And(filters ...OrganizationFilter) OrganizationFilter {
	return andFilter{filters: filters}
}

// AndFilter combines many filters
type andFilter struct {
	filters []OrganizationFilter
}

// ExtendQuery extends a query to filter by all the filters in this AndFilter.
func (a andFilter) ExtendQuery(b squirrel.SelectBuilder) squirrel.SelectBuilder {
	for _, f := range a.filters {
		b = f.ExtendQuery(b)
	}
	return b
}

// Matches all the filters in this AndFilter.
func (a andFilter) Matches(o users.Organization) bool {
	for _, f := range a.filters {
		if !f.Matches(o) {
			return false
		}
	}
	return true
}

// Organization defines a filter for listing organizations.
// Supported filters
// - id:<organization-id>
// - instance:<external-id>
// - feature:<feature-flag>
type Organization struct {
	ID           string
	Instance     string
	FeatureFlags []string
	Extra        OrganizationFilter

	Search string
	Page   int32
}

// NewOrganizationFromRequest extracts filter values from the request.
func NewOrganizationFromRequest(r *http.Request) Organization {
	q := parseQuery(r.FormValue("query"))
	return Organization{
		ID:           q.filters["id"],
		Instance:     q.filters["instance"],
		FeatureFlags: q.featureFlags,
		Search:       strings.Join(q.search, " "),
		Page:         pageValue(r),
		Extra:        q.extra,
	}
}

// Matches says whether the given organization matches this filter.
//
// Must be kept in sync with ExtendQuery.
func (o Organization) Matches(org users.Organization) bool {
	if o.ID != "" && org.ID != o.ID {
		return false
	}
	if o.Instance != "" && org.ExternalID != o.Instance {
		return false
	}
	if o.Search != "" && !strings.Contains(org.Name, o.Search) {
		return false
	}
	for _, wantFlag := range o.FeatureFlags {
		if !org.HasFeatureFlag(wantFlag) {
			return false
		}
	}
	if o.Extra != nil && !o.Extra.Matches(org) {
		return false
	}
	return true
}

// ExtendQuery applies the filter to the query builder.
//
// Must be kept in sync with Matches.
func (o Organization) ExtendQuery(b squirrel.SelectBuilder) squirrel.SelectBuilder {
	if o.Page > 0 {
		b = b.Limit(resultsPerPage).Offset(uint64((o.Page - 1) * resultsPerPage))
	}
	if o.Search != "" {
		b = b.Where("lower(organizations.name) LIKE ?",
			fmt.Sprint("%", strings.ToLower(o.Search), "%"))
	}

	// `AND` all feature flags
	for _, f := range o.FeatureFlags {
		b = b.Where("?=ANY(feature_flags)", f)
	}

	if o.Extra != nil {
		b = o.Extra.ExtendQuery(b)
	}

	where := squirrel.Eq{}
	if o.ID != "" {
		where["id"] = o.ID
	}
	if o.Instance != "" {
		where["external_id"] = o.Instance
	}

	return b.Where(where)
}
