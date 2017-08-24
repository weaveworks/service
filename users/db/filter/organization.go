package filter

import (
	"net/http"
)

// Organization defines a filter for listing organizations.
type Organization struct {
	Query string
	Page  int32
}

// NewOrganization extracts filter values from the request.
func NewOrganization(r *http.Request) Organization {
	return Organization{
		Query: r.FormValue("query"),
		Page:  pageValue(r),
	}
}
