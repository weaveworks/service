package filter

import (
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

type query struct {
	filters      map[string]string
	featureFlags []string
	search       []string
}

// pageValue extracts the `page` form value of the request. It also
// clamps it to (1, ∞).
func pageValue(r *http.Request) int32 {
	page, _ := strconv.ParseInt(r.FormValue("page"), 10, 32)
	if page <= 0 {
		page = 1
	}
	return int32(page)
}

// parseQuery extracts filters and search from the `query` form value.
// It supports `<key>:<value>` for exact matches as well as `is:<key>`
// for boolean toggles, and `has:<feature>` for feature flags.
func parseQuery(qs string) query {
	q := query{filters: map[string]string{}}
	for _, p := range strings.Fields(qs) {
		if strings.Contains(p, queryFilterDelim) {
			kv := strings.SplitN(p, queryFilterDelim, 2)
			switch kv[0] {
			case "is":
				q.filters[kv[1]] = "true"
			case "has":
				q.featureFlags = append(q.featureFlags, kv[1])
			default:
				q.filters[kv[0]] = kv[1]
			}
		} else {
			q.search = append(q.search, p)
		}
	}

	return q
}
