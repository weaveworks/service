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
		{"api_users_gcp_subscribe", "POST", "/api/users/gcp/subscribe", a.authenticateUser(a.gcpSubscribe)},
		{"api_users_gcp_sso_login", "GET", "/api/users/gcp/sso/login/{externalAccountID}", a.authenticateUser(a.gcpSSOLogin)},
		// The same as api_users_signup, but exempt from CRSF in authfe and authenticated via header here
		{"api_users_signup_webhook", "POST", "/api/users/signup_webhook", a.authenticateWebhook(a.signup)},

		// This is the link the UI hits when the user visits the link from the
		// login email. Doesn't need to handle any attachment, or providers, since
		// it *only* handles email logins.
		{"api_users_login", "GET", "/api/users/login", a.login},

		// Logs the current user out (just deletes the session cookie)
		{"api_users_logout", "GET", "/api/users/logout", a.logout},

		// This is the first endpoint the UI hits to see if the user is logged in.
		{"api_users_lookup", "GET", "/api/users/lookup", a.authenticateUser(a.publicLookup)},

		{"api_users_teams", "GET", "/api/users/teams", a.authenticateUser(a.listTeams)},
		{"api_users_teams_teamExternalID_delete", "DELETE", "/api/users/teams/{teamExternalID}", a.authenticateUser(a.deleteTeam)},

		// Used by the launcher agent to get the external instance ID using a token
		{"api_users_org_token_lookup", "GET", "/api/users/org/lookup", a.authenticateProbe(a.orgLookup)},

		// Basic view and management of an organization
		{"api_users_generateOrgName", "GET", "/api/users/generateOrgName", a.authenticateUser(a.generateOrgExternalID)},
		{"api_users_generateOrgID", "GET", "/api/users/generateOrgID", a.authenticateUser(a.generateOrgExternalID)},
		{"api_users_org_create", "POST", "/api/users/org", a.authenticateUser(a.createOrg)},
		{"api_users_org_orgExternalID", "GET", "/api/users/org/{orgExternalID}", a.authenticateUser(a.org)},
		{"api_users_org_orgExternalID_update", "PUT", "/api/users/org/{orgExternalID}", a.authenticateUser(a.updateOrg)},
		{"api_users_org_orgExternalID_delete", "DELETE", "/api/users/org/{orgExternalID}", a.authenticateUser(a.deleteOrg)},
		{"api_users_org_service_status", "GET", "/api/users/org/{orgExternalID}/status", a.authenticateUser(a.getOrgServiceStatus)},

		// Used to list and manage organization access (invites)
		{"api_users_org_orgExternalID_users", "GET", "/api/users/org/{orgExternalID}/users", a.authenticateUser(a.listOrganizationUsers)},
		{"api_users_org_orgExternalID_inviteUser", "POST", "/api/users/org/{orgExternalID}/users", a.authenticateUser(a.inviteUser)},
		{"api_users_org_orgExternalID_removeUser", "DELETE", "/api/users/org/{orgExternalID}/users/{userEmail}", a.authenticateUser(a.removeUser)},

		// Internal stuff for our internal usage, internally.
		{"root", "GET", "/admin/users", a.admin},
		{"admin_users_organizations", "GET", "/admin/users/organizations", a.adminListOrganizations},
		{"admin_users_gcp_externalAccoutnID_subscriptions", "GET", "/admin/users/gcp/{externalAccountID}/subscriptions", a.adminGCPListSubscriptions},
		{"admin_users_organizations_orgExternalID_users", "GET", "/admin/users/organizations/{orgExternalID}/users", a.adminListUsersForOrganization},
		{"admin_users_organizations_orgExternalID_users_userID", "POST", "/admin/users/organizations/{orgExternalID}/users/{userID}/remove", a.adminRemoveUserFromOrganization},
		{"admin_users_organizations_orgExternalID", "POST", "/admin/users/organizations/{orgExternalID}", a.adminChangeOrgFields},
		{"admin_users_organizations_orgExternalID_trial", "POST", "/admin/users/organizations/{orgExternalID}/trial", a.adminTrial},
		{"admin_users_organizations_orgExternalID_delete", "POST", "/admin/users/organizations/{orgExternalID}/remove", a.adminDeleteOrganization},
		{"admin_users_users", "GET", "/admin/users/users", a.adminListUsers},
		{"admin_users_users_userID_admin", "POST", "/admin/users/users/{userID}/admin", a.adminMakeUserAdmin},
		{"admin_users_users_userID_become", "POST", "/admin/users/users/{userID}/become", a.adminBecomeUser},
		{"admin_users_users_userID_delete", "POST", "/admin/users/users/{userID}/remove", a.adminDeleteUser},
		{"admin_users_users_userID_logins_provider_token", "GET", "/admin/users/users/{userID}/logins/{provider}/token", a.adminGetUserToken},
		{"admin_users_users_userID_organizations", "GET", "/admin/users/users/{userID}/organizations", a.adminListOrganizationsForUser},

		// HealthCheck
		{"healthcheck", "GET", "/api/users/healthcheck", a.healthcheck},
	} {
		r.Handle(route.path, route.handler).Methods(route.method).Name(route.name)
	}
}
