package main

import (
	"net/http"
	"path/filepath"

	"github.com/gorilla/mux"
	"github.com/weaveworks/common/middleware"
)

// Routable denotes things capable of registering routes
type Routable interface {
	RegisterRoutes(*mux.Router)
}

// PrefixRoutable registers routes which match prefixes, and can be below a MiddlewarePrefix
type PrefixRoutable interface {
	Routable
	Handler() http.Handler
	MatchPathPrefix(r *mux.Router, parentPrefix string) *mux.Route
	AbsolutePrefix(parentPrefix string) string
}

/***************************************************************************/

// Path routable says "map this path to this handler"
type Path struct {
	path    string
	handler http.Handler
}

// RegisterRoutes adds routes to a router
func (p Path) RegisterRoutes(r *mux.Router) {
	r.Path(p.path).Name(middleware.MakeLabelValue(p.path)).Handler(p.handler)
}

/***************************************************************************/

// Prefix maps a path prefix to a handler
type Prefix struct {
	prefix  string
	handler http.Handler
}

// RegisterRoutes adds routes to a router
func (p Prefix) RegisterRoutes(r *mux.Router) {
	p.MatchPathPrefix(r, "").Handler(p.handler)
}

// AbsolutePrefix returns the absolute prefix for the route
func (p Prefix) AbsolutePrefix(parentPrefix string) string {
	return filepath.Join(parentPrefix, p.prefix)
}

// MatchPathPrefix builds a route on a router, matching this prefix
func (p Prefix) MatchPathPrefix(r *mux.Router, parentPrefix string) *mux.Route {
	path := p.AbsolutePrefix(parentPrefix)
	return r.PathPrefix(path).Name(middleware.MakeLabelValue(path))
}

// Handler returns the handler for the route
func (p Prefix) Handler() http.Handler {
	return p.handler
}

// Matchables converts a []Prefix into []PrefixRoutable
// you can't cast because golang, and you can't copy() either
func Matchables(prefixes []Prefix) []PrefixRoutable {
	m := make([]PrefixRoutable, len(prefixes))
	for i := range prefixes {
		m[i] = prefixes[i]
	}
	return m
}

/***************************************************************************/

// PrefixMethods maps a path prefix with certain request methods to a handler
type PrefixMethods struct {
	prefix  string
	methods []string
	handler http.Handler
}

// RegisterRoutes adds routes to a router
func (p PrefixMethods) RegisterRoutes(r *mux.Router) {
	p.MatchPathPrefix(r, "").Handler(p.handler)
}

// AbsolutePrefix returns the absolute prefix for the route
func (p PrefixMethods) AbsolutePrefix(parentPrefix string) string {
	return filepath.Join(parentPrefix, p.prefix)
}

// MatchPathPrefix builds a route on a router, matching this prefix and HTTP methods
func (p PrefixMethods) MatchPathPrefix(r *mux.Router, parentPrefix string) *mux.Route {
	path := p.AbsolutePrefix(parentPrefix)
	return r.PathPrefix(path).Methods(p.methods...).Name(middleware.MakeLabelValue(path))
}

// Handler returns the handler for the route
func (p PrefixMethods) Handler() http.Handler {
	return p.handler
}

/***************************************************************************/

// A MiddlewarePrefix says "for each of these PrefixRoutables, add our path prefix
// and (optionally) wrap the handlers with this middleware.
type MiddlewarePrefix struct {
	prefix string
	routes []PrefixRoutable
	mid    middleware.Interface
}

// RegisterRoutes adds routes to a router
func (p MiddlewarePrefix) RegisterRoutes(r *mux.Router) {
	if p.mid == nil {
		p.mid = middleware.Identity
	}
	for _, route := range p.routes {
		route.MatchPathPrefix(r, p.prefix).Handler(p.mid.Wrap(route.Handler()))
	}
}

// AbsolutePrefixes returns the path prefixes
// this should probably be moved to "routable" if/when we accept nested prefixes
func (p MiddlewarePrefix) AbsolutePrefixes() []string {
	var result []string
	for _, route := range p.routes {
		path := route.AbsolutePrefix(p.prefix)
		result = append(result, filepath.Clean(path))
	}
	return result
}

// HostnameSpecific says "only match when the hostname matches"
type HostnameSpecific struct {
	hostname string
	routes   []PrefixRoutable
	mid      middleware.Interface
}

// RegisterRoutes adds routes to a router
func (h HostnameSpecific) RegisterRoutes(r *mux.Router) {
	if h.mid == nil {
		h.mid = middleware.Identity
	}
	for _, route := range h.routes {
		r.Host(h.hostname).Handler(h.mid.Wrap(route.Handler()))
	}
}
