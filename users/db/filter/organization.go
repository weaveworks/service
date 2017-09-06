package filter

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/Masterminds/squirrel"
)

// Organization defines a filter for listing organizations.
type Organization struct {
	Instance string `json:"instance,omitempty"`
	ID       string `json:"id,omitempty"`

	Search string `json:"-"`
	Page   int32  `json:"-"`
}

// NewOrganization extracts filter values from the request.
func NewOrganization(r *http.Request) Organization {
	o := Organization{}
	o.Search = parseQuery(r, &o)
	o.Page = pageValue(r)
	return o
}

// ExtendQuery applies the filter to the query builder.
func (o Organization) ExtendQuery(b squirrel.SelectBuilder) squirrel.SelectBuilder {
	if o.Page > 0 {
		b = b.Limit(resultsPerPage).Offset(uint64((o.Page - 1) * resultsPerPage))
	}
	if o.Search != "" {
		b = b.Where("lower(organizations.name) LIKE ?",
			fmt.Sprint("%", strings.ToLower(o.Search), "%"))
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
