package common

import "net/url"

func FlattenQueryParams(params url.Values) map[string]string {
	result := make(map[string]string)
	// pass on any query parameters (ignoring duplicate keys)
	for key, values := range params {
		result[key] = values[0]
	}
	return result
}
