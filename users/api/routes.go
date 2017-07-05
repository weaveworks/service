package api

import (
	"net/http"

	"github.com/gorilla/mux"
)

// RegisterRoutes registers the users API HTTP routes to the provided Router.
func (a *API) RegisterRoutes(r *mux.Router) {
	for _, route := range []struct {
		name, method, path string
		handler            http.HandlerFunc
	}{
		// Used by the UI to determine all available login providers
		{"api_users_logins", "GET", "/api/users/logins", a.listLoginProviders},

		// Used by the UI for /account, to determine which providers are already
		// attached to the current user
		{"api_users_attached_logins", "GET", "/api/users/attached_logins", a.authenticateUser(a.listAttachedLoginProviders)},

		// Attaches a new login provider to the current user. If no current user is
		// logged in, one will be looked up via email, or we will create one (if no
		// matching email is found).
		//
		// This endpoint will then set the session cookie, return a json object
		// with some fields like:
		//  { "firstLogin": true, "attach": true, "userCreated": true }
		{"api_users_logins_provider_attach", "GET", "/api/users/logins/{provider}/attach", a.attachLoginProvider},
		// Detaches the given provider from the current user
		{"api_users_logins_provider_detach", "POST", "/api/users/logins/{provider}/detach", a.authenticateUser(a.detachLoginProvider)},

		// Finds/Creates a user account with a given email, and emails them a new
		// login link
		{"api_users_signup", "POST", "/api/users/signup", a.signup},

		// This is the link the UI hits when the user visits the link from the
		// login email. Doesn't need to handle any attachment, or providers, since
		// it *only* handles email logins.
		{"api_users_login", "GET", "/api/users/login", a.login},

		// Logs the current user out (just deletes the session cookie)
		{"api_users_logout", "GET", "/api/users/logout", a.logout},

		// This is the first endpoint the UI hits to see if the user is logged in.
		{"api_users_lookup", "GET", "/api/users/lookup", a.authenticateUser(a.publicLookup)},

		// Basic view and management of an organization
		{"api_users_generateOrgName", "GET", "/api/users/generateOrgName", a.authenticateUser(a.generateOrgExternalID)},
		{"api_users_generateOrgID", "GET", "/api/users/generateOrgID", a.authenticateUser(a.generateOrgExternalID)},
		{"api_users_org_create", "POST", "/api/users/org", a.authenticateUser(a.createOrg)},
		{"api_users_org_orgExternalID", "GET", "/api/users/org/{orgExternalID}", a.authenticateUser(a.org)},
		{"api_users_org_orgExternalID_update", "PUT", "/api/users/org/{orgExternalID}", a.authenticateUser(a.updateOrg)},
		{"api_users_org_orgExternalID_delete", "DELETE", "/api/users/org/{orgExternalID}", a.authenticateUser(a.deleteOrg)},

		// Used to list and manage organization access (invites)
		{"api_users_org_orgExternalID_users", "GET", "/api/users/org/{orgExternalID}/users", a.authenticateUser(a.listOrganizationUsers)},
		{"api_users_org_orgExternalID_inviteUser", "POST", "/api/users/org/{orgExternalID}/users", a.authenticateUser(a.inviteUser)},
		{"api_users_org_orgExternalID_deleteUser", "DELETE", "/api/users/org/{orgExternalID}/users/{userEmail}", a.authenticateUser(a.deleteUser)},

		// Internal stuff for our internal usage, internally.
		{"root", "GET", "/admin/users", a.admin},
		{"admin_users_organizations", "GET", "/admin/users/organizations", a.listOrganizations},
		{"admin_users_organizations_orgExternalID", "POST", "/admin/users/organizations/{orgExternalID}", a.changeOrgField},
		{"admin_users_pardot", "GET", "/admin/users/marketing_refresh", a.marketingRefresh},
		{"admin_users_users", "GET", "/admin/users/users", a.listUsers},
		{"admin_users_users_userID_admin", "POST", "/admin/users/users/{userID}/admin", a.makeUserAdmin},
		{"admin_users_users_userID_become", "POST", "/admin/users/users/{userID}/become", a.becomeUser},
		{"admin_users_users_userID_logins_provider_token", "GET", "/admin/users/users/{userID}/logins/{provider}/token", a.getUserToken},
		{"admin_users_users_userID_organizations", "GET", "/admin/users/users/{userID}/organizations", a.listOrganizationsForUser},
	} {
		r.Handle(route.path, route.handler).Methods(route.method).Name(route.name)
	}
}
