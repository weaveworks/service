package filter

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

const (
	// Delimiter for query WHERE conditions as in `id:3`
	queryFilterDelim = ":"

	// Page size for paginated listings
	resultsPerPage = 30
)

// pageValue extracts the `page` form value of the request. It also
// clamps it to (1, ∞).
func pageValue(r *http.Request) int32 {
	page, _ := strconv.ParseInt(r.FormValue("page"), 10, 32)
	if page <= 0 {
		page = 1
	}
	return int32(page)
}

// parseQuery extracts filters from the `query` form value.
// It unmarshals the filters of the form `key:value` to the
// fields of `dst` and returns the global search term.
func parseQuery(r *http.Request, dst interface{}) string {
	query := r.FormValue("query")

	parts := strings.Fields(query)
	search := []string{}
	filters := map[string]string{}
	for _, p := range parts {
		if strings.Contains(p, queryFilterDelim) {
			kv := strings.SplitN(p, queryFilterDelim, 2)
			filters[kv[0]] = kv[1]
		} else {
			search = append(search, p)
		}
	}

	// Copy over to dst
	bs, _ := json.Marshal(filters)
	json.Unmarshal(bs, &dst)

	return strings.Join(search, " ")
}
