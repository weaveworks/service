package filter

import (
	"fmt"
	"strings"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/weaveworks/service/users"
)

// User filters users.
type User interface {
	Filter
	// MatchesUser checks whether a user matches this filter.
	MatchesUser(users.User) bool
}

// UsersByID finds users by their database IDs.
type UsersByID []string

// MatchesUser users whose ID is in the list.
func (f UsersByID) MatchesUser(u users.User) bool {
	for _, id := range f {
		if id == u.ID {
			return true
		}
	}
	return false
}

// Where returns the query to filter for users whose ID is in the list.
func (f UsersByID) Where() squirrel.Sqlizer {
	return squirrel.Eq(map[string]interface{}{
		"users.id": []string(f),
	})
}

// SearchEmail finds users whose email contains the given string.
type SearchEmail string

// MatchesUser users whose email addresses contain the given string.
func (s SearchEmail) MatchesUser(u users.User) bool {
	return strings.Contains(u.Email, string(s))
}

// Where returns the query to filter for emails that contain the given string.
func (s SearchEmail) Where() squirrel.Sqlizer {
	return squirrel.Expr("lower(users.email) LIKE ?",
		fmt.Sprint("%", strings.ToLower(string(s)), "%"))
}

// LoggedInSince finds users who have logged in since a given time.
type LoggedInSince time.Time

// MatchesUser users who have logged in since the given time.
func (s LoggedInSince) MatchesUser(u users.User) bool {
	return u.LastLoginAt.After(time.Time(s))
}

// Where returns the query to filter for users who have logged in since the given time.
func (s LoggedInSince) Where() squirrel.Sqlizer {
	return squirrel.Gt{"last_login_at": time.Time(s)}
}

// NotLoggedInSince finds users who have logged in since a given time.
// Squirrel has no NOT filter, so we can't have a general NOT filter either
type NotLoggedInSince time.Time

// MatchesUser users who have not logged in since the given time.
func (s NotLoggedInSince) MatchesUser(u users.User) bool {
	return time.Time(s).After(u.LastLoginAt)
}

// Where returns the query to filter for users who have not logged in since the given time.
func (s NotLoggedInSince) Where() squirrel.Sqlizer {
	return squirrel.LtOrEq{"last_login_at": time.Time(s)}
}

// Admin finds users who are admins (or who aren't).
type Admin bool

// MatchesUser users with the right admin status.
func (a Admin) MatchesUser(u users.User) bool {
	return u.Admin == bool(a)
}

// Where returns the query to filter for admin status.
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
