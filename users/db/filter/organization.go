package filter

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/Masterminds/squirrel"
	"github.com/weaveworks/service/users"
)

// Organization defines a filter for listing organizations.
// Supported filters
// - id:<organization-id>
// - instance:<external-id>
// - has:<feature-flag>
type Organization struct {
	ID           string
	Instance     string
	FeatureFlags []string

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
WANT:
	for _, wantFlag := range o.FeatureFlags {
		for _, hasFlag := range org.FeatureFlags {
			if hasFlag == wantFlag {
				continue WANT
			}
		}
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

	where := squirrel.Eq{}
	if o.ID != "" {
		where["id"] = o.ID
	}
	if o.Instance != "" {
		where["external_id"] = o.Instance
	}

	return b.Where(where)
}
