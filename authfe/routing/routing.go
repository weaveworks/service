package routing

import (
	"net/http"
	"path/filepath"

	"github.com/gorilla/mux"
	"github.com/weaveworks/common/middleware"
)

type Routable interface {
	Add(*mux.Router)
}

type PrefixMatchable interface {
	Path(prefix string) string
	MatchPathPrefix(*mux.Router, string) *mux.Route

	Handler() http.Handler
}

// A path routable says "map this path to this handler"
type Path struct {
	path    string
	handler http.Handler
}

func (p Path) Add(r *mux.Router) {
	r.Path(p.path).Name(middleware.MakeLabelValue(p.path)).Handler(p.handler)
}

// Prefix maps a path prefix to a handler
type Prefix struct {
	path    string
	handler http.Handler
}

func (p Prefix) Path(prefix string) string {
	return filepath.Join(prefix, p.path)
}

func (p Prefix) MatchPathPrefix(r *mux.Router, prefix string) *mux.Route {
	path := p.Path(prefix)
	return r.PathPrefix(path).Name(middleware.MakeLabelValue(path))
}

func (p Prefix) Handler() http.Handler {
	return p.handler
}

func Matchables(prefixes []Prefix) []PrefixMatchable {
	m := make([]PrefixMatchable, len(prefixes))
	for i := range prefixes {
		m[i] = prefixes[i]
	}
	return m
}

// PrefixMethods maps a path prefix with certain request methods to a handler
type PrefixMethods struct {
	path    string
	methods []string
	handler http.Handler
}

func (p PrefixMethods) Path(prefix string) string {
	return filepath.Join(prefix, p.path)
}

func (p PrefixMethods) MatchPathPrefix(r *mux.Router, prefix string) *mux.Route {
	path := p.Path(prefix)
	return r.PathPrefix(path).Methods(p.methods...).Name(middleware.MakeLabelValue(path))
}

func (p PrefixMethods) Handler() http.Handler {
	return p.handler
}

// A MiddlewarePrefix says "for each of these path routables, add this path prefix
// and (optionally) wrap the handlers with this middleware.
type MiddlewarePrefix struct {
	prefix string
	routes []PrefixMatchable
	mid    middleware.Interface
}

func (p MiddlewarePrefix) Add(r *mux.Router) {
	if p.mid == nil {
		p.mid = middleware.Identity
	}
	for _, route := range p.routes {
		route.MatchPathPrefix(r, p.prefix).Handler(p.mid.Wrap(route.Handler()))
	}
}

// this should probably be moved to "routable" if/when we accept nested prefixes
func (p MiddlewarePrefix) AbsolutePrefixes() []string {
	var result []string
	for _, route := range p.routes {
		result = append(result, filepath.Clean(route.Path(p.prefix)))
	}
	return result
}
