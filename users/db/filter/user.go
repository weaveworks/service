package filter

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/Masterminds/squirrel"
)

// User defines a filter for listing users.
type User struct {
	AdminOnly bool
	Query     string
	Page      int32
}

// NewUser extracts filter values from the request.
func NewUser(r *http.Request) User {
	return User{
		AdminOnly: r.FormValue("admin") == "true",
		Query:     r.FormValue("query"),
		Page:      pageValue(r),
	}
}

// ExtendQuery applies the filter to the query builder.
func (u User) ExtendQuery(b squirrel.SelectBuilder) squirrel.SelectBuilder {
	if u.AdminOnly {
		b = b.Where("users.admin = true")
	}
	if u.Query != "" {
		b = b.Where("lower(userggs.email) LIKE ?",
			fmt.Sprint("%", strings.ToLower(u.Query), "%"))
	}
	if u.Page > 0 {
		b = b.Limit(resultsPerPage).Offset(uint64((u.Page - 1) * resultsPerPage))
	}

	return b
}
