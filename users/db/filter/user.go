package filter

import (
	"net/http"
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
