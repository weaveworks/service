package storage

import (
	"net/url"

	"github.com/Masterminds/squirrel"
)

// Filter lets us be selective when loading objects from the database
type Filter interface {
	// Apply this filter to an individual item
	Item(interface{}) bool

	// Apply this filter to an SQL select query
	Select(squirrel.SelectBuilder) squirrel.SelectBuilder
}

// Filters is a collection of filters, which we can parse from a query string.
type Filters map[string]func([]string) Filter

// Parse the filters from an http query string
func (f Filters) parse(params url.Values) []Filter {
	fs := []Filter{}
	for param, factory := range f {
		if values := params[param]; len(values) > 0 {
			fs = append(fs, factory(params[param]))
		}
	}
	return fs
}

// InFilter lets us check if a value is in some set
type InFilter struct {
	Allowed  func(item interface{}) bool
	SQLField string
	Value    interface{}
	SQLJoins []string
}

// Item applies this filter to an individual item
func (f InFilter) Item(item interface{}) bool {
	return f.Allowed(item)
}

// Select applies this filter to an SQL select query
func (f InFilter) Select(q squirrel.SelectBuilder) squirrel.SelectBuilder {
	q = q.Where(squirrel.Eq{f.SQLField: f.Value})
	for _, t := range f.SQLJoins {
		q = q.Join(t)
	}
	return q
}

// And is a logical and of some filters. Only returns objects matching all
// filters.
type And []Filter

// Item applies this filter to an individual item
func (a And) Item(item interface{}) bool {
	for _, f := range a {
		if !f.Item(item) {
			return false
		}
	}
	return true
}

// Select applies this filter to an SQL select query
func (a And) Select(q squirrel.SelectBuilder) squirrel.SelectBuilder {
	for _, f := range a {
		q = f.Select(q)
	}
	return q
}
