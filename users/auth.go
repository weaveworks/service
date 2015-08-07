package main

import "strings"

type credentials struct {
	Realm  string
	Params map[string]string
}

func parseAuthHeader(header string) (*credentials, bool) {
	for _, realm := range []string{"Basic", "Bearer"} {
		prefix := realm + " "
		if strings.HasPrefix(header, prefix) {
			k := strings.ToLower(realm)
			return &credentials{
				Realm:  realm,
				Params: map[string]string{k: strings.TrimPrefix(header, prefix)},
			}, true
		}
	}
	i := strings.IndexByte(header, ' ')
	if i == -1 {
		return nil, false
	}

	c := &credentials{Realm: header[:i], Params: map[string]string{}}
	for _, field := range strings.Split(header[i+1:], ",") {
		if i := strings.IndexByte(field, '='); i == -1 {
			c.Params[field] = ""
		} else {
			c.Params[field[:i]] = field[i+1:]
		}
	}
	return c, true
}
