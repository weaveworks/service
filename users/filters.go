package main

import (
	"net/url"

	"github.com/Masterminds/squirrel"
)

type filter interface {
	// Apply this filter to an individual item
	Item(interface{}) bool

	// Apply this filter to an SQL select query
	Select(squirrel.SelectBuilder) squirrel.SelectBuilder
}

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

type inFilter struct {
	Allowed  func(item interface{}) bool
	SQLField string
	Value    interface{}
	SQLJoins []string
}

func (f inFilter) Item(item interface{}) bool {
	return f.Allowed(item)
}

func (f inFilter) Select(q squirrel.SelectBuilder) squirrel.SelectBuilder {
	q = q.Where(squirrel.Eq{f.SQLField: f.Value})
	for _, t := range f.SQLJoins {
		q = q.Join(t)
	}
	return q
}

type and []filter

func (a and) Item(item interface{}) bool {
	for _, f := range a {
		if !f.Item(item) {
			return false
		}
	}
	return true
}

func (a and) Select(q squirrel.SelectBuilder) squirrel.SelectBuilder {
	for _, f := range a {
		q = f.Select(q)
	}
	return q
}
