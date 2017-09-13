package filter

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/Masterminds/squirrel"
	"github.com/weaveworks/service/users"
)

const (
	// Ignored is for when we don't care if a boolean value is true or false
	Ignored = Has(iota)
	// Absent is for when we only want things with the boolean value set to false
	Absent
	// Present is for when we only want things with the boolean value set to true.
	Present
)

// Has is for when we want to check whether an organization has a field set.
// XXX: jml doesn't like the name 'Has' -- any suggestions?
type Has uint8

// ExtendQuery returns a new query that filters by whether the column is set or not.
//
// Must be kept in sync with Matches.
func (h Has) ExtendQuery(b squirrel.SelectBuilder, column string) squirrel.SelectBuilder {
	switch h {
	case Ignored:
		return b
	case Absent:
		return b.Where(map[string]interface{}{column: nil})
	case Present:
		return b.Where(fmt.Sprintf("%s IS NOT NULL", column))
	default:
		panic(fmt.Sprintf("Unrecognized state: %#v", h))
	}
}

// Matches checks whether the organization matches this filter.
//
// Must be kept in sync with ExtendQuery.
func (h Has) Matches(o users.Organization) bool {
	switch h {
	case Ignored:
		return true
	case Absent:
		return o.ZuoraAccountNumber == ""
	case Present:
		return o.ZuoraAccountNumber != ""
	default:
		panic(fmt.Sprintf("Unrecognized state: %#v", h))
	}
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
	ZuoraAccount Has

	Search string
	Page   int32
}

// NewOrganization extracts filter values from the request.
func NewOrganization(r *http.Request) Organization {
	q := parseQuery(r.FormValue("query"))
	return Organization{
		ID:           q.filters["id"],
		Instance:     q.filters["instance"],
		FeatureFlags: q.featureFlags,
		Search:       strings.Join(q.search, " "),
		Page:         pageValue(r),
		ZuoraAccount: Ignored,
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
	if !o.ZuoraAccount.Matches(org) {
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

	b = o.ZuoraAccount.ExtendQuery(b, "zuora_account_number")

	where := squirrel.Eq{}
	if o.ID != "" {
		where["id"] = o.ID
	}
	if o.Instance != "" {
		where["external_id"] = o.Instance
	}

	return b.Where(where)
}
