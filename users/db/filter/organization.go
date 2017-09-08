package filter

import (
	"encoding/json"
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
	query := parseQuery(r.FormValue("query"))
	o := Organization{}

	// Copy filters over to the organization's fields.
	// No error checking because we don't care, empty fields are fine.
	bs, _ := json.Marshal(query.filters)
	json.Unmarshal(bs, &o)

	o.Search = strings.Join(query.search, " ")
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
