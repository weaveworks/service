package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"golang.org/x/net/context"

	"github.com/weaveworks/common/logging"
	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/client"
	"github.com/weaveworks/service/users/db/filter"
	"github.com/weaveworks/service/users/login"
	"github.com/weaveworks/service/users/render"
)

func (a *API) admin(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/html")
	fmt.Fprintf(w, `
<!doctype html>
<html>
	<head><title>User service</title></head>
	<body>
		<h1>User service</h1>
		<ul>
			<li><a href="/admin/users/users">Users</a></li>
			<li><a href="/admin/users/organizations">Organizations</a></li>
			<li><a href="/admin/users/marketing_refresh">Sync User-Creation with marketing integrations</a></li>
		</ul>
	</body>
</html>
`)
}

type listUsersView struct {
	Users []privateUserView `json:"organizations"`
}

type privateUserView struct {
	ID           string `json:"id"`
	Email        string `json:"email"`
	CreatedAt    string `json:"created_at"`
	FirstLoginAt string `json:"first_login_at"`
	Admin        bool   `json:"admin"`
}

func (a *API) listUsers(w http.ResponseWriter, r *http.Request) {
	page := filter.ParsePageValue(r.FormValue("page"))
	query := r.FormValue("query")
	f := filter.And(filter.ParseUserQuery(query), filter.Page(page))
	users, err := a.db.ListUsers(r.Context(), f)
	if err != nil {
		render.Error(w, r, err)
		return
	}
	switch render.Format(r) {
	case render.FormatJSON:
		view := listUsersView{}
		for _, user := range users {
			view.Users = append(view.Users, privateUserView{
				ID:           user.ID,
				Email:        user.Email,
				CreatedAt:    user.FormatCreatedAt(),
				FirstLoginAt: user.FormatFirstLoginAt(),
				Admin:        user.Admin,
			})
		}
		render.JSON(w, http.StatusOK, view)
	default: // render.FormatHTML
		b, err := a.templates.Bytes("list_users.html", map[string]interface{}{
			"Users":    users,
			"Query":    r.FormValue("query"),
			"Page":     page,
			"NextPage": page + 1,
		})
		if err != nil {
			render.Error(w, r, err)
			return
		}
		if _, err := w.Write(b); err != nil {
			logging.With(r.Context()).Warn("list users: %v", err)
		}
	}
}

func (a *API) listUsersForOrganization(w http.ResponseWriter, r *http.Request) {
	orgID, ok := mux.Vars(r)["orgExternalID"]
	if !ok {
		render.Error(w, r, users.ErrNotFound)
		return
	}
	_, err := a.db.FindOrganizationByID(r.Context(), orgID)
	if err != nil {
		render.Error(w, r, users.ErrNotFound)
		return
	}
	users, err := a.db.ListOrganizationUsers(r.Context(), orgID)
	if err != nil {
		render.Error(w, r, err)
		return
	}

	b, err := a.templates.Bytes("list_users.html", map[string]interface{}{
		"Users":         users,
		"OrgExternalID": orgID,
	})
	if err != nil {
		render.Error(w, r, err)
		return
	}
	if _, err := w.Write(b); err != nil {
		logging.With(r.Context()).Warn("list users: %v", err)
	}
}

func (a *API) listOrganizations(w http.ResponseWriter, r *http.Request) {
	page := filter.ParsePageValue(r.FormValue("page"))
	query := r.FormValue("query")
	f := filter.And(filter.ParseOrgQuery(query), filter.Page(page))
	organizations, err := a.db.ListOrganizations(r.Context(), f)
	if err != nil {
		render.Error(w, r, err)
		return
	}

	b, err := a.templates.Bytes("list_organizations.html", map[string]interface{}{
		"Organizations": organizations,
		"Query":         r.FormValue("query"),
		"Page":          page,
		"NextPage":      page + 1,
		"Message":       r.FormValue("msg"),
	})
	if err != nil {
		render.Error(w, r, err)
		return
	}
	if _, err := w.Write(b); err != nil {
		logging.With(r.Context()).Warn("list organizations: %v", err)
	}
}

func (a *API) listOrganizationsForUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID, ok := vars["userID"]
	if !ok {
		render.Error(w, r, users.ErrNotFound)
		return
	}
	user, err := a.db.FindUserByID(r.Context(), userID)
	if err != nil {
		render.Error(w, r, err)
		return
	}
	organizations, err := a.db.ListOrganizationsForUserIDs(r.Context(), userID)
	if err != nil {
		render.Error(w, r, err)
		return
	}

	b, err := a.templates.Bytes("list_organizations.html", map[string]interface{}{
		"Organizations": organizations,
		"UserEmail":     user.Email,
	})
	if err != nil {
		render.Error(w, r, err)
		return
	}
	if _, err := w.Write(b); err != nil {
		logging.With(r.Context()).Warn("list organizations: %v", err)
	}
}

func (a *API) changeOrgField(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	orgExternalID, ok := vars["orgExternalID"]
	if !ok {
		render.Error(w, r, users.ErrNotFound)
		return
	}
	field := r.FormValue("field")
	value := r.FormValue("value")

	var err error
	switch field {
	case "FirstSeenConnectedAt":
		now := time.Now()
		err = a.db.SetOrganizationFirstSeenConnectedAt(r.Context(), orgExternalID, &now)
	case "DenyUIFeatures":
		deny := value == "on"
		err = a.db.SetOrganizationDenyUIFeatures(r.Context(), orgExternalID, deny)
	case "DenyTokenAuth":
		deny := value == "on"
		err = a.db.SetOrganizationDenyTokenAuth(r.Context(), orgExternalID, deny)
	case "FeatureFlags":
		err = a.setOrganizationFeatureFlags(r.Context(), orgExternalID, strings.Fields(value))
	default:
		err = users.ValidationErrorf("Invalid field %v", field)
		return
	}

	if err != nil {
		render.Error(w, r, err)
		return
	}

	msg := fmt.Sprintf("Saved `%s` for %s", field, orgExternalID)
	http.Redirect(w, r, "/admin/users/organizations?msg="+url.QueryEscape(msg), http.StatusFound)
}

func (a *API) marketingRefresh(w http.ResponseWriter, r *http.Request) {
	users, err := a.db.ListUsers(r.Context(), filter.All)
	if err != nil {
		render.Error(w, r, err)
		return
	}

	for _, user := range users {
		a.marketingQueues.UserCreated(user.Email, user.CreatedAt)
	}
}

// setOrganizationFeatureFlags updates feature flags of an organization.
func (a *API) setOrganizationFeatureFlags(ctx context.Context, orgExternalID string, flags []string) error {
	uniqueFlags := map[string]struct{}{}
	for _, f := range flags {
		uniqueFlags[f] = struct{}{}
	}
	var sortedFlags []string
	for f := range uniqueFlags {
		sortedFlags = append(sortedFlags, f)
	}
	sort.Strings(sortedFlags)

	// Keep track whether we are about to enable the billing flag
	var billingEngaged bool
	var orgName string
	if _, ok := uniqueFlags[users.BillingFeatureFlag]; ok {
		org, err := a.db.FindOrganizationByID(ctx, orgExternalID)
		if err != nil {
			return err
		}
		orgName = org.Name
		billingEngaged = !org.HasFeatureFlag(users.BillingFeatureFlag)
	}

	err := a.db.SetFeatureFlags(ctx, orgExternalID, sortedFlags)
	if err != nil {
		return err
	}

	// For post-creation enabling of billing, we extend the trial period
	// starting today and send members an email
	if billingEngaged {
		expires := time.Now().Add(users.TrialExtensionDuration)
		err = a.db.UpdateOrganization(ctx, orgExternalID, users.OrgWriteView{TrialExpiresAt: &expires})
		if err != nil {
			return err
		}

		members, err := a.db.ListOrganizationUsers(ctx, orgExternalID)
		if err != nil {
			return err
		}
		err = a.emailer.TrialExtendedEmail(members, orgExternalID, orgName, expires)
		if err != nil {
			return err
		}
	}

	return nil
}

func (a *API) makeUserAdmin(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID, ok := vars["userID"]
	if !ok {
		render.Error(w, r, users.ErrNotFound)
		return
	}
	admin := r.FormValue("admin") == "true"
	if err := a.MakeUserAdmin(r.Context(), userID, admin); err != nil {
		render.Error(w, r, err)
		return
	}
	redirectTo := r.FormValue("redirect_to")
	if redirectTo == "" {
		redirectTo = "/admin/users/users"
	}
	http.Redirect(w, r, redirectTo, http.StatusFound)
}

func (a *API) becomeUser(w http.ResponseWriter, r *http.Request) {
	logging.With(r.Context()).Info(r)
	vars := mux.Vars(r)
	userID, ok := vars["userID"]
	if !ok {
		render.Error(w, r, users.ErrNotFound)
		return
	}
	u, err := a.db.FindUserByID(r.Context(), userID)
	if err != nil {
		render.Error(w, r, err)
		return
	}
	session, err := a.sessions.Get(r)
	if err != nil {
		return
	}
	impersonatingUserID := session.ImpersonatingUserID
	if impersonatingUserID != "" {
		// Impersonation already in progress ... unusual, but hang on to existing impersonator ID
	} else {
		// No existing impersonation ... store current user is impersonator
		impersonatingUserID = session.UserID
	}
	if err := a.sessions.Set(w, r, u.ID, impersonatingUserID); err != nil {
		render.Error(w, r, users.ErrInvalidAuthenticationData)
		return
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

func (a *API) getUserToken(w http.ResponseWriter, r *http.Request) {
	// Get User ID from path
	vars := mux.Vars(r)
	userID, ok := vars["userID"]
	if !ok {
		render.Error(w, r, users.ErrProviderParameters)
		return
	}
	provider, ok := vars["provider"]
	if !ok {
		render.Error(w, r, users.ErrProviderParameters)
		return
	}

	// Does user exist?
	_, err := a.db.FindUserByID(r.Context(), userID)
	if err != nil {
		render.Error(w, r, err)
		return
	}

	// Get logins for user
	logins, err := a.db.ListLoginsForUserIDs(r.Context(), userID)
	if err != nil {
		render.Error(w, r, err)
		return
	}
	l, err := getSpecificLogin(provider, logins)
	if err != nil {
		render.Error(w, r, err)
		return
	}

	// Parse session information to get token
	tok, err := parseTokenFromSession(l.Session)
	if err != nil {
		render.Error(w, r, err)
		return
	}
	render.JSON(w, 200, client.ProviderToken{
		Token: tok,
	})
	return
}

func getSpecificLogin(login string, logins []*login.Login) (*login.Login, error) {
	for _, l := range logins {
		if l.Provider == login {
			return l, nil
		}
	}
	return nil, users.ErrLoginNotFound
}

func parseTokenFromSession(session json.RawMessage) (string, error) {
	b, err := session.MarshalJSON()
	if err != nil {
		return "", err
	}
	var sess struct {
		Token struct {
			AccessToken string `json:"access_token"`
		} `json:"token"`
	}
	err = json.Unmarshal(b, &sess)
	return sess.Token.AccessToken, err
}

// MakeUserAdmin makes a user an admin
func (a *API) MakeUserAdmin(ctx context.Context, userID string, admin bool) error {
	return a.db.SetUserAdmin(ctx, userID, admin)
}
