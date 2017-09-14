package filter

import (
	"strconv"
	"strings"

	"github.com/Masterminds/squirrel"
	"github.com/weaveworks/service/users"
)

const (
	// Delimiter for query WHERE conditions as in `id:3`
	queryFilterDelim = ":"

	// Page size for paginated listings
	resultsPerPage = 30
)

var (
	// All includes everything.
	All = And()
)

// Filter filters things.
type Filter interface {
	// ExtendQuery extends a query to filter by something.
	ExtendQuery(squirrel.SelectBuilder) squirrel.SelectBuilder
}

// And combines many filters.
func And(filters ...Filter) AndFilter {
	return AndFilter(filters)
}

// AndFilter combines many filters
type AndFilter []Filter

// ExtendQuery extends a query to filter by all the filters in this AndFilter.
func (a AndFilter) ExtendQuery(b squirrel.SelectBuilder) squirrel.SelectBuilder {
	for _, f := range a {
		b = f.ExtendQuery(b)
	}
	return b
}

// MatchesOrg matches all the filters in this AndFilter.
func (a AndFilter) MatchesOrg(o users.Organization) bool {
	for _, f := range a {
		orgMatcher := f.(Organization)
		if !orgMatcher.MatchesOrg(o) {
			return false
		}
	}
	return true
}

// MatchesUser matches all the filters in this AndFilter.
func (a AndFilter) MatchesUser(u users.User) bool {
	for _, f := range a {
		userMatcher := f.(UserFilter)
		if !userMatcher.MatchesUser(u) {
			return false
		}
	}
	return true
}

// Page shows only a single page of results.
type Page int32 // XXX: Should be uint64

// MatchesOrg says whether the given organization matches this filter.
//
// We don't implement pagination for queries against the in-memory database,
// so just lets everything through.
func (p Page) MatchesOrg(_ users.Organization) bool {
	return true
}

// MatchesUser says whether the given user matches this filter.
//
// We don't implement pagination for queries against the in-memory database,
// so just lets everything through.
func (p Page) MatchesUser(_ users.User) bool {
	return true
}

// ExtendQuery applies the filter to the query builder.
func (p Page) ExtendQuery(b squirrel.SelectBuilder) squirrel.SelectBuilder {
	page := int32(p)
	if page > 0 {
		b = b.Limit(resultsPerPage).Offset(uint64((page - 1) * resultsPerPage))
	}
	return b
}

// ParsePageValue parses the `page` form value of the request. It also
// clamps it to (1, ∞).
func ParsePageValue(pageStr string) int32 {
	page, _ := strconv.ParseInt(pageStr, 10, 32)
	if page <= 0 {
		page = 1
	}
	return int32(page)
}

// ParseOrgQuery extracts filters and search from the `query` form
// value. It supports `<key>:<value>` for exact matches as well as `is:<key>`
// for boolean toggles, and `feature:<feature>` for feature flags.
func ParseOrgQuery(qs string) Organization {
	filters := []Filter{}
	search := []string{}
	for _, p := range strings.Fields(qs) {
		if strings.Contains(p, queryFilterDelim) {
			kv := strings.SplitN(p, queryFilterDelim, 2)
			switch kv[0] {
			case "feature":
				filters = append(filters, HasFeatureFlag(kv[1]))
			case "has":
				switch kv[1] {
				case "zuora":
					filters = append(filters, ZuoraAccount(true))
				}
			case "!has":
				switch kv[1] {
				case "zuora":
					filters = append(filters, ZuoraAccount(false))
				}
			case "id":
				filters = append(filters, ID(kv[1]))
			case "instance":
				filters = append(filters, ExternalID(kv[1]))
			default:
				search = append(search, p)
			}
		} else {
			search = append(search, p)
		}
	}
	if len(search) > 0 {
		filters = append(filters, SearchName(strings.Join(search, " ")))
	}
	return And(filters...)
}
