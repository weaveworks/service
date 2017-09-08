package filter

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/Masterminds/squirrel"
)

// User defines a filter for listing users.
// Supported filters
// - is:admin
type User struct {
	Admin bool

	Search string
	Page   int32
}

// NewUser extracts filter values from the request.
func NewUser(r *http.Request) User {
	q := parseQuery(r.FormValue("query"))
	return User{
		Admin:  q.filters["admin"] == "true",
		Search: strings.Join(q.search, " "),
		Page:   pageValue(r),
	}
}

// ExtendQuery applies the filter to the query builder.
func (u User) ExtendQuery(b squirrel.SelectBuilder) squirrel.SelectBuilder {
	if u.Page > 0 {
		b = b.Limit(resultsPerPage).Offset(uint64((u.Page - 1) * resultsPerPage))
	}
	if u.Search != "" {
		b = b.Where("lower(users.email) LIKE ?",
			fmt.Sprint("%", strings.ToLower(u.Search), "%"))
	}

	if u.Admin {
		b = b.Where("users.admin = true")
	}

	return b
}
