package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/weaveworks/service/common/orgs"

	"github.com/gorilla/mux"

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
			logging.With(r.Context()).Warnf("list users: %v", err)
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
		logging.With(r.Context()).Warnf("list users: %v", err)
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
		logging.With(r.Context()).Warnf("list organizations: %v", err)
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
		logging.With(r.Context()).Warnf("list organizations: %v", err)
	}
}

func (a *API) changeOrgFields(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	orgExternalID, ok := vars["orgExternalID"]
	if !ok {
		render.Error(w, r, users.ErrNotFound)
		return
	}

	// Single value `field=foo, value=bar`
	if r.FormValue("field") != "" {
		if err := a.setOrganizationField(r.Context(), orgExternalID, r.FormValue("field"), r.FormValue("value")); err != nil {
			render.Error(w, r, err)
			return
		}
	} else { // Multi value `foo=bar, moo=zar`
		fields := [...]string{"FeatureFlags", orgs.RefuseDataAccess, orgs.RefuseDataUpload}
		var errs []string
		for _, field := range fields {
			if err := a.setOrganizationField(r.Context(), orgExternalID, field, r.FormValue(field)); err != nil {
				errs = append(errs, err.Error())
			}
		}

		if len(errs) > 0 {
			render.Error(w, r, errors.New(strings.Join(errs, "; ")))
			return
		}
	}

	msg := fmt.Sprintf("Saved config for %s", orgExternalID)
	http.Redirect(w, r, "/admin/users/organizations?msg="+url.QueryEscape(msg), http.StatusFound)
}

func (a *API) setOrganizationField(ctx context.Context, orgExternalID, field, value string) error {
	var err error
	switch field {
	case "FirstSeenConnectedAt":
		now := time.Now()
		err = a.db.SetOrganizationFirstSeenConnectedAt(ctx, orgExternalID, &now)
	case orgs.RefuseDataAccess:
		deny := value == "on"
		err = a.db.SetOrganizationRefuseDataAccess(ctx, orgExternalID, deny)
	case orgs.RefuseDataUpload:
		deny := value == "on"
		err = a.db.SetOrganizationRefuseDataUpload(ctx, orgExternalID, deny)
	case "FeatureFlags":
		err = a.setOrgFeatureFlags(ctx, orgExternalID, strings.Fields(value))
	default:
		err = users.ValidationErrorf("Invalid field %v", field)
	}
	return err
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
