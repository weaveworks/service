package api

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/weaveworks/common/middleware"

	"github.com/weaveworks/service/common"
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

		// Listing and managing API tokens
		{"api_users_tokens", "GET", "/api/users/tokens", a.authenticateUser(a.listAPITokens)},
		{"api_users_tokens_create", "POST", "/api/users/tokens", a.authenticateUser(a.createAPIToken)},
		{"api_users_tokens_delete", "DELETE", "/api/users/tokens/{token}", a.authenticateUser(a.deleteAPIToken)},

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

		// The users service client (i.e. our other services) use these to
		// authenticate the admin/user/probe.
		{"private_api_users_admin", "GET", "/private/api/users/admin", a.authenticateUser(a.lookupAdmin)},
		{"private_api_users_lookup_orgExternalID", "GET", "/private/api/users/lookup/{orgExternalID}", a.authenticateUser(a.lookupOrg)},
		{"private_api_users_lookup", "GET", "/private/api/users/lookup", a.authenticateProbe(a.lookupUsingToken)},
		{"private_api_users_lookup_user", "GET", "/private/api/users/lookup_user", a.authenticateUser(a.lookupUser)},

		// Internal stuff for our internal usage, internally.
		{"root", "GET", "/", a.admin},
		{"private_api_organizations", "GET", "/private/api/organizations", a.listOrganizations},
		{"private_api_organizations_orgExternalID", "GET", "/private/api/organizations/{orgExternalID}", a.adminShowOrganization},
		{"private_api_organizations_orgExternalID_featureFlags", "POST", "/private/api/organizations/{orgExternalID}/featureflags", a.setOrgFeatureFlags},
		{"private_api_pardot", "GET", "/private/api/marketing_refresh", a.marketingRefresh},
		{"private_api_users", "GET", "/private/api/users", a.listUsers},
		{"private_api_users_userID_admin", "POST", "/private/api/users/{userID}/admin", a.makeUserAdmin},
		{"private_api_users_userID_logins_provider_token", "GET", "/private/api/users/{userID}/logins/{provider}/token", a.getUserToken},
	} {
		r.Handle(route.path, route.handler).Methods(route.method).Name(route.name)
	}
	return middleware.Merge(
		middleware.Log{
			LogSuccess: a.logSuccess,
		},
		middleware.Instrument{
			RouteMatcher: r,
			Duration:     common.RequestDuration,
		},
	).Wrap(r)
}
