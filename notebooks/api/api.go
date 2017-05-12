package api

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/weaveworks/service/notebooks/db"
	"github.com/weaveworks/service/users"
)

// API implements the users api.
type API struct {
	db db.DB
	http.Handler
	usersClient users.UsersClient
}

// New creates a new API.
func New(database db.DB, usersClient users.UsersClient) *API {
	a := &API{db: database, usersClient: usersClient}
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
		{"api_list_notebooks", "GET", "/api/prom/notebooks", a.listNotebooks},
		{"api_create_notebook", "POST", "/api/prom/notebooks", a.createNotebook},

		{"api_get_notebook", "GET", "/api/prom/notebooks/{notebookID}", a.getNotebook},
		{"api_update_notebook", "PUT", "/api/prom/notebooks/{notebookID}", a.updateNotebook},
		{"api_delete_notebook", "DELETE", "/api/prom/notebooks/{notebookID}", a.deleteNotebook},
	} {
		r.Handle(route.path, route.handler).Methods(route.method).Name(route.name)
	}
}
