package main

import (
	"net/url"
)

type filter interface{}

type filters map[string]func([]string) filter

// Parse the filters from an http query string
func (f filters) parse(params url.Values) []filter {
	fs := []filter{}
	for param, factory := range f {
		if values := params[param]; len(values) > 0 {
			fs = append(fs, factory(params[param]))
		}
	}
	return fs
}
