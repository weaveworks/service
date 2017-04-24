package api

import (
	"net/http"

	"github.com/gorilla/mux"
	db "github.com/weaveworks/service/prom/db/dynamo"
)

// API implements the users api.
type API struct {
	db db.DB
	http.Handler
}

// NewAPI creates a new API.
func NewAPI(database db.DB) *API {
	a := &API{db: database}
	r := mux.NewRouter()
	a.RegisterRoutes(r)
	a.Handler = r
	return a
}

// RegisterRoutes registers the prom API HTTP routes to the provided Router.
func (a *API) RegisterRoutes(r *mux.Router) {
	for _, route := range []struct {
		name, method, path string
		handler            http.HandlerFunc
	}{
		{"api_get_all_notebooks", "GET", "/api/prom/notebooks", a.getAllNotebooks},
	} {
		r.Handle(route.path, route.handler).Methods(route.method).Name(route.name)
	}
}
