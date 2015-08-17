package instrument

import (
	"strings"
	"unicode"
)

// MakeLabelValue converts a Gorilla mux path to a string suitable for use in
// a Prometheus label value.
func MakeLabelValue(path string) string {
	// Convert non-alnums to underscores.
	var a []rune
	for _, r := range strings.ToLower(path) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			a = append(a, r)
			continue
		}
		a = append(a, '_')
	}
	// Trim leading and trailing underscores, and compress duplicate
	// underscores to a single underscore.
	result := strings.Trim(string(a), "_")
	for {
		if next := dedupe.Replace(result); next != result {
			result = next
			continue
		}
		break
	}
	// Special case.
	if result == "" {
		result = "root"
	}
	return result
}
