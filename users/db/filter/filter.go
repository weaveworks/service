package filter

import (
	"strings"

	"github.com/Masterminds/squirrel"
	"github.com/weaveworks/service/users"
)

const (
	// Delimiter for query WHERE conditions as in `id:3`
	queryFilterDelim = ":"
)

var (
	// All includes everything.
	All = And()
)

// Filter filters things.
type Filter interface {
	// Where returns the query for this filter.
	Where() squirrel.Sqlizer
}

// And combines many filters.
func And(filters ...Filter) AndFilter {
	return AndFilter(filters)
}

// AndFilter combines many filters
type AndFilter []Filter

// Where returns the query to filter by all the filters in this AndFilter.
func (a AndFilter) Where() squirrel.Sqlizer {
	wheres := []squirrel.Sqlizer{}
	for _, f := range a {
		wheres = append(wheres, f.Where())
	}
	return squirrel.And(wheres)
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
		userMatcher := f.(User)
		if !userMatcher.MatchesUser(u) {
			return false
		}
	}
	return true
}

// MatchesTeam matches all the filters in this AndFilter.
func (a AndFilter) MatchesTeam(t users.Team) bool {
	for _, f := range a {
		matcher := f.(Team)
		if !matcher.MatchesTeam(t) {
			return false
		}
	}
	return true
}

// OrFilter requires at least one filter to pass.
type OrFilter []Filter

// Or combines filters into an OR'ed condition.
func Or(filters ...Filter) OrFilter {
	return OrFilter(filters)
}

// Where returns the query to filter by at least one of the filters in this OrFilter.
func (o OrFilter) Where() squirrel.Sqlizer {
	ors := []squirrel.Sqlizer{}
	for _, f := range o {
		ors = append(ors, f.Where())
	}
	return squirrel.Or(ors)
}

// MatchesOrg matches at least one of the filters in this OrFilter.
func (o OrFilter) MatchesOrg(org users.Organization) bool {
	for _, f := range o {
		orgMatcher := f.(Organization)
		if orgMatcher.MatchesOrg(org) {
			return true
		}
	}
	return false
}

// MatchesUser matches at least one of the filters in this OrFilter.
func (o OrFilter) MatchesUser(u users.User) bool {
	for _, f := range o {
		userMatcher := f.(User)
		if userMatcher.MatchesUser(u) {
			return true
		}
	}
	return false
}

// MatchesTeam matches at least one of the filters in this OrFilter.
func (o OrFilter) MatchesTeam(t users.Team) bool {
	for _, f := range o {
		matcher := f.(Team)
		if matcher.MatchesTeam(t) {
			return true
		}
	}
	return false
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
				case "gcp":
					filters = append(filters, GCP(true))
				}
			case "!has":
				switch kv[1] {
				case "zuora":
					filters = append(filters, ZuoraAccount(false))
				case "gcp":
					filters = append(filters, GCP(false))
				}
			case "id":
				filters = append(filters, ID(kv[1]))
			case "instance", "external":
				filters = append(filters, ExternalID(kv[1]))
			case "token":
				filters = append(filters, ProbeToken(kv[1]))
			case "version":
				filters = append(filters, PlatformVersion(kv[1]))
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

// ParseUserQuery extracts filters and search from the 'query' form value.
func ParseUserQuery(qs string) User {
	filters := []Filter{}
	search := []string{}
	for _, p := range strings.Fields(qs) {
		if strings.Contains(p, queryFilterDelim) {
			kv := strings.SplitN(p, queryFilterDelim, 2)
			switch kv[0] {
			case "id":
				filters = append(filters, UsersByID(strings.Split(kv[1], ",")))
			case "is":
				switch kv[1] {
				case "admin":
					filters = append(filters, Admin(true))
				}
			default:
				search = append(search, p)
			}
		} else {
			search = append(search, p)
		}
	}
	if len(search) > 0 {
		filters = append(filters, SearchEmail(strings.Join(search, " ")))
	}
	return And(filters...)
}

// ParseTeamQuery extracts filters and search from the 'query' form value.
func ParseTeamQuery(qs string) Team {
	filters := []Filter{}
	search := []string{}
	for _, p := range strings.Fields(qs) {
		if strings.Contains(p, queryFilterDelim) {
			kv := strings.SplitN(p, queryFilterDelim, 2)
			switch kv[0] {
			case "id":
				filters = append(filters, TeamID(kv[1]))
			case "team", "external":
				filters = append(filters, TeamExternalID(kv[1]))
			default:
				search = append(search, p)
			}
		} else {
			search = append(search, p)
		}
	}
	if len(search) > 0 {
		filters = append(filters, TeamName(strings.Join(search, " ")))
	}
	return And(filters...)
}
