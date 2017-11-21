package filter

import "strconv"

// Page size for paginated listings
const ResultsPerPage = 30

// ParsePageValue parses the `page` form value of the request. It also
// clamps it to (1, ∞).
func ParsePageValue(pageStr string) uint64 {
	page, _ := strconv.ParseInt(pageStr, 10, 32)
	if page <= 0 {
		page = 1
	}
	return uint64(page)
}

