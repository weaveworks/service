package filter

import (
	"fmt"
	"strings"

	"github.com/Masterminds/squirrel"
	"github.com/weaveworks/service/users"
)

// User filters users.
type User interface {
	Filter
	// MatchesUser checks whether a user matches this filter.
	MatchesUser(users.User) bool
}

// SearchEmail finds users whose email contains the given string.
type SearchEmail string

// MatchesUser users whose email addresses contain the given string.
func (s SearchEmail) MatchesUser(u users.User) bool {
	return strings.Contains(u.Email, string(s))
}

// ExtendQuery extends a query to also filter for emails that contain the given string.
func (s SearchEmail) Where() squirrel.Sqlizer {
	return squirrel.Expr("lower(users.email) LIKE ?",
		fmt.Sprint("%", strings.ToLower(string(s)), "%"))
}

// Admin finds users who are admins (or who aren't).
type Admin bool

// MatchesUser users with the right admin status.
func (a Admin) MatchesUser(u users.User) bool {
	return u.Admin == bool(a)
}

// ExtendQuery extends a query to filter for admin status.
func (a Admin) Where() squirrel.Sqlizer {
	return squirrel.Eq{"users.admin": bool(a)}
}

// ParseUserQuery extracts filters and search from the 'query' form value.
func ParseUserQuery(qs string) User {
	filters := []Filter{}
	search := []string{}
	for _, p := range strings.Fields(qs) {
		if strings.Contains(p, queryFilterDelim) {
			kv := strings.SplitN(p, queryFilterDelim, 2)
			switch kv[0] {
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
