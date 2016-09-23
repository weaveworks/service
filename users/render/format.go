package render

import (
	"net/http"
	"strings"
)

// These are the supported response formats
const (
	FormatHTML = "text/html"
	FormatJSON = "application/json"
	FormatText = "text/plain"
)

// Format checks the accept header format for a request
// TODO: Use a better mime-type parser
func Format(r *http.Request) string {
	header := r.Header.Get("Accept")
	for _, f := range []string{
		FormatHTML,
		FormatJSON,
		FormatText,
	} {
		if header == f || strings.HasPrefix(header, f) {
			return f
		}
	}

	// HTML is the default
	return FormatHTML
}
