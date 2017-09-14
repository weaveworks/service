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
type ZuoraAccount bool

// ExtendQuery extends a query to filter by Zuora account.
func (z ZuoraAccount) ExtendQuery(b squirrel.SelectBuilder) squirrel.SelectBuilder {
	if bool(z) {
		return b.Where("zuora_account_number IS NOT NULL")
	}
	return b.Where(map[string]interface{}{"zuora_account_number": nil})
}

// Matches checks whether the organization matches this filter.
//
// Must be kept in sync with ExtendQuery.
func (z ZuoraAccount) Matches(o users.Organization) bool {
	if bool(z) {
		return o.ZuoraAccountNumber != ""
	}
	return o.ZuoraAccountNumber == ""
}

// TrialExpiredBy filters for organizations whose trials had expired by a
// given date.
type TrialExpiredBy time.Time

// ExtendQuery extends a query to filter by trial expiry.
//
// Must be kept in sync with Matches.
func (t TrialExpiredBy) ExtendQuery(b squirrel.SelectBuilder) squirrel.SelectBuilder {
	return b.Where("organizations.trial_expires_at < ?", time.Time(t))
}

// Matches checks whether an organization matches this filter.
//
// Must be kept in sync with ExtendQuery.
func (t TrialExpiredBy) Matches(o users.Organization) bool {
	return o.TrialExpiresAt.Before(time.Time(t))
}

// TrialActiveAt filters for organizations whose trials were active at given
// date.
type TrialActiveAt time.Time

// ExtendQuery extends a query to filter by trial expiry.
//
// Must be kept in sync with Matches.
func (t TrialActiveAt) ExtendQuery(b squirrel.SelectBuilder) squirrel.SelectBuilder {
	return b.Where("organizations.trial_expires_at > ?", time.Time(t))
}

// Matches checks whether an organization matches this filter.
//
// Must be kept in sync with ExtendQuery.
func (t TrialActiveAt) Matches(o users.Organization) bool {
	return o.TrialExpiresAt.After(time.Time(t))
}

// HasFeatureFlag filters for organizations that has the given feature flag.
type HasFeatureFlag string

// ExtendQuery extends a query to filter by feature flag.
//
// Must be kept in sync with Matches.
func (f HasFeatureFlag) ExtendQuery(b squirrel.SelectBuilder) squirrel.SelectBuilder {
	return b.Where("?=ANY(feature_flags)", string(f))
}

// Matches checks whether an organization matches this filter.
//
// Must be kept in sync with ExtendQuery.
func (f HasFeatureFlag) Matches(o users.Organization) bool {
	return o.HasFeatureFlag(string(f))
}

// And combines many filters.
func And(filters ...OrganizationFilter) OrganizationFilter {
	return andFilter(filters)
}

// AndFilter combines many filters
type andFilter []OrganizationFilter

// ExtendQuery extends a query to filter by all the filters in this AndFilter.
func (a andFilter) ExtendQuery(b squirrel.SelectBuilder) squirrel.SelectBuilder {
	for _, f := range a {
		b = f.ExtendQuery(b)
	}
	return b
}

// Matches all the filters in this AndFilter.
func (a andFilter) Matches(o users.Organization) bool {
	for _, f := range a {
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
	ID       string
	Instance string
	Extra    OrganizationFilter

	Search string
	Page   int32
}

// NewOrganizationFromRequest extracts filter values from the request.
func NewOrganizationFromRequest(r *http.Request) Organization {
	q := parseQuery(r.FormValue("query"))
	return Organization{
		ID:       q.filters["id"],
		Instance: q.filters["instance"],
		Search:   strings.Join(q.search, " "),
		Page:     pageValue(r),
		Extra:    q.extra,
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
