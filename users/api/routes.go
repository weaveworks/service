package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/justinas/nosurf"
	"github.com/weaveworks/scope/common/middleware"

	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/render"
)

func (a *API) routes() http.Handler {
	r := mux.NewRouter()
	for _, route := range []struct {
		name, method, path string
		handler            http.HandlerFunc
	}{
		// Used by the UI to determine all available login providers
		{"api_users_logins", "GET", "/api/users/logins", a.listLoginProviders},

		// Used by the UI for /account, to determine which providers are already
		// attached to the current user
		{"api_users_attached_logins", "GET", "/api/users/attached_logins", a.authenticated(a.listAttachedLoginProviders)},

		// Attaches a new login provider to the current user. If no current user is
		// logged in, one will be looked up via email, or we will create one (if no
		// matching email is found).
		//
		// This endpoint will then set the session cookie, return a json object
		// with some fields like:
		//  { "firstLogin": true, "attach": true, "userCreated": true }
		{"api_users_logins_provider_attach", "GET", "/api/users/logins/{provider}/attach", a.attachLoginProvider},
		// Detaches the given provider from the current user
		{"api_users_logins_provider_detach", "POST", "/api/users/logins/{provider}/detach", a.authenticated(a.detachLoginProvider)},

		// Finds/Creates a user account with a given email, and emails them a new
		// login link
		{"api_users_signup", "POST", "/api/users/signup", a.signup},

		// This is the link the UI hits when the user visits the link from the
		// login email. Doesn't need to handle any attachment, or providers, since
		// it *only* handles email logins.
		{"api_users_login", "GET", "/api/users/login", a.login},

		// Logs the current user out (just deletes the session cookie)
		{"api_users_logout", "GET", "/api/users/logout", a.authenticated(a.logout)},

		// This is the first endpoint the UI hits to see if the user is logged in.
		{"api_users_lookup", "GET", "/api/users/lookup", a.authenticated(a.publicLookup)},

		// Basic view and management of an organization
		{"api_users_generateOrgName", "GET", "/api/users/generateOrgName", a.authenticated(a.generateOrgExternalID)},
		{"api_users_generateOrgID", "GET", "/api/users/generateOrgID", a.authenticated(a.generateOrgExternalID)},
		{"api_users_org_create", "POST", "/api/users/org", a.authenticated(a.createOrg)},
		{"api_users_org_orgExternalID", "GET", "/api/users/org/{orgExternalID}", a.authenticated(a.org)},
		{"api_users_org_orgExternalID_update", "PUT", "/api/users/org/{orgExternalID}", a.authenticated(a.updateOrg)},
		{"api_users_org_orgExternalID_delete", "DELETE", "/api/users/org/{orgExternalID}", a.authenticated(a.deleteOrg)},

		// Used to list and manage organization access (invites)
		{"api_users_org_orgExternalID_users", "GET", "/api/users/org/{orgExternalID}/users", a.authenticated(a.listOrganizationUsers)},
		{"api_users_org_orgExternalID_inviteUser", "POST", "/api/users/org/{orgExternalID}/users", a.authenticated(a.inviteUser)},
		{"api_users_org_orgExternalID_deleteUser", "DELETE", "/api/users/org/{orgExternalID}/users/{userEmail}", a.authenticated(a.deleteUser)},

		// The users service client (i.e. our other services) use these to
		// authenticate the admin/user/probe.
		{"private_api_users_admin", "GET", "/private/api/users/admin", a.authenticated(a.lookupAdmin)},
		{"private_api_users_lookup_orgExternalID", "GET", "/private/api/users/lookup/{orgExternalID}", a.authenticated(a.lookupOrg)},
		{"private_api_users_lookup", "GET", "/private/api/users/lookup", a.lookupUsingToken},

		// Internal stuff for our internal usage, internally.
		{"loadgen", "GET", "/loadgen", func(w http.ResponseWriter, _ *http.Request) { fmt.Fprintf(w, "OK") }},
		{"root", "GET", "/", a.admin},
		{"private_api_users", "GET", "/private/api/users", a.listUsers},
		{"private_api_pardot", "GET", "/private/api/pardot", a.pardotRefresh},
		{"private_api_users_userID_admin", "POST", "/private/api/users/{userID}/admin", a.makeUserAdmin},
	} {
		r.Handle(route.path, route.handler).Methods(route.method).Name(route.name)
	}
	return middleware.Merge(
		middleware.Logging,
		middleware.Instrument{
			RouteMatcher: r,
			Duration:     users.RequestDuration,
		},
		middleware.Func(csrf),
	).Wrap(r)
}

func (a *API) authenticated(handler func(*users.User, http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, err := a.sessions.Get(r)
		if err != nil {
			render.Error(w, r, err)
			return
		}

		// User actions always go through this endpoint because
		// app-mapper checks the authentication endpoint eevry time.
		// We use this to tell pardot about login activity.
		a.pardotClient.UserAccess(u.Email, time.Now())

		handler(u, w, r)
	})
}

// Make csrf stuff (via nosurf) available in this handler, and set the csrf
// token cookie in any responses.
func csrf(handler http.Handler) http.Handler {
	h := nosurf.New(handler)
	h.SetBaseCookie(http.Cookie{
		MaxAge:   nosurf.MaxAge,
		HttpOnly: true,
		Path:     "/",
	})
	// We don't use nosurf's csrf checking. We only use it to generate & compare
	// tokens.
	h.ExemptFunc(func(r *http.Request) bool { return true })
	return h
}
