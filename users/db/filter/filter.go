package filter

import (
	"strconv"
	"strings"
)

const (
	// Delimiter for query WHERE conditions as in `id:3`
	queryFilterDelim = ":"

	// Page size for paginated listings
	resultsPerPage = 30
)

// ParsePageValue parses the `page` form value of the request. It also
// clamps it to (1, ∞).
func ParsePageValue(pageStr string) int32 {
	page, _ := strconv.ParseInt(pageStr, 10, 32)
	if page <= 0 {
		page = 1
	}
	return int32(page)
}

// ParseOrgQuery extracts filters and search from the `query` form
// value. It supports `<key>:<value>` for exact matches as well as `is:<key>`
// for boolean toggles, and `feature:<feature>` for feature flags.
func ParseOrgQuery(qs string) Organization {
	filters := []Organization{}
	search := []string{}
	for _, p := range strings.Fields(qs) {
		if strings.Contains(p, queryFilterDelim) {
			kv := strings.SplitN(p, queryFilterDelim, 2)
			switch kv[0] {
			case "feature":
				filters = append(filters, HasFeatureFlag(kv[1]))
			case "has":
				switch kv[1] {
				case "zuora":
					filters = append(filters, ZuoraAccount(true))
				}
			case "!has":
				switch kv[1] {
				case "zuora":
					filters = append(filters, ZuoraAccount(false))
				}
			case "id":
				filters = append(filters, ID(kv[1]))
			case "instance":
				filters = append(filters, ExternalID(kv[1]))
			default:
				search = append(search, p)
			}
		} else {
			search = append(search, p)
		}
	}
	if len(search) > 0 {
		filters = append(filters, SearchName(strings.Join(search, " ")))
	}
	return And(filters...)
}
