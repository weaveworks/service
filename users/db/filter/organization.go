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

// ID filters for organizations with exactly this ID.
type ID string

// ExtendQuery extends a query to filter by ID.
//
// Must be kept in sync with Matches.
func (i ID) ExtendQuery(b squirrel.SelectBuilder) squirrel.SelectBuilder {
	return b.Where(map[string]string{"id": string(i)})
}

// Matches checks whether an organization matches this filter.
//
// Must be kept in sync with ExtendQuery.
func (i ID) Matches(o users.Organization) bool {
	return o.ID == string(i)
}

// ExternalID filters for organizations with exactly this external ID.
type ExternalID string

// ExtendQuery extends a query to filter by ID.
//
// Must be kept in sync with Matches.
func (e ExternalID) ExtendQuery(b squirrel.SelectBuilder) squirrel.SelectBuilder {
	return b.Where(map[string]string{"external_id": string(e)})
}

// Matches checks whether an organization matches this filter.
//
// Must be kept in sync with ExtendQuery.
func (e ExternalID) Matches(o users.Organization) bool {
	return o.ExternalID == string(e)
}

// SearchName finds organizations that have names that contain a string.
type SearchName string

// ExtendQuery extends a query to filter by having names that match our search.
func (s SearchName) ExtendQuery(b squirrel.SelectBuilder) squirrel.SelectBuilder {
	return b.Where("lower(organizations.name) LIKE ?",
		fmt.Sprint("%", strings.ToLower(string(s)), "%"))
}

// Matches checks whether an organization matches this filter.
//
// Must be kept in sync with ExtendQuery.
func (s SearchName) Matches(o users.Organization) bool {
	return strings.Contains(o.Name, string(s))
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

// Page shows only a single page of results.
type Page int32 // XXX: Should be uint64

// Matches says whether the given organization matches this filter.
//
// We don't implement pagination for queries against the in-memory database,
// so just lets everything through.
func (p Page) Matches(org users.Organization) bool {
	return true
}

// ExtendQuery applies the filter to the query builder.
//
// Must be kept in sync with Matches.
func (p Page) ExtendQuery(b squirrel.SelectBuilder) squirrel.SelectBuilder {
	page := int32(p)
	if page > 0 {
		b = b.Limit(resultsPerPage).Offset(uint64((page - 1) * resultsPerPage))
	}
	return b
}

// Organization defines a filter for listing organizations.
// Supported filters
// - id:<organization-id>
// - instance:<external-id>
// - feature:<feature-flag>
type Organization struct {
	Extra OrganizationFilter

	Page int32
}

// NewOrganizationFromRequest extracts filter values from the request.
func NewOrganizationFromRequest(r *http.Request) Organization {
	filters := parseOrgQuery(r.FormValue("query"))
	page := pageValue(r)
	filters = And(filters, Page(page))
	return Organization{
		Page:  pageValue(r),
		Extra: filters,
	}
}

// Matches says whether the given organization matches this filter.
//
// Must be kept in sync with ExtendQuery.
func (o Organization) Matches(org users.Organization) bool {
	if o.Extra != nil {
		return o.Extra.Matches(org)
	}
	return true
}

// ExtendQuery applies the filter to the query builder.
//
// Must be kept in sync with Matches.
func (o Organization) ExtendQuery(b squirrel.SelectBuilder) squirrel.SelectBuilder {
	if o.Extra != nil {
		b = o.Extra.ExtendQuery(b)
	}
	return b
}
