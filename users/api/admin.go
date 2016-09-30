package api

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"

	"github.com/weaveworks/service/users"
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
			<li><a href="private/api/users">Users</a></li>
			<li><a href="private/api/organizations">Organizations</a></li>
			<li><a href="private/api/marketing_refresh">Sync User-Creation with marketing integrations</a></li>
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
	users, err := a.db.ListUsers()
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
			"Users": users,
		})
		if err != nil {
			render.Error(w, r, err)
			return
		}
		if _, err := w.Write(b); err != nil {
			logrus.Warn("list users: %v", err)
		}
	}
}

type listOrganizationsView struct {
	Organizations []privateOrgView `json:"organizations"`
}

type privateOrgView struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	CreatedAt          string   `json:"created_at"`
	FirstProbeUpdateAt string   `json:"first_probe_update_at,omitempty"`
	FeatureFlags       []string `json:"feature_flags,omitempty"`
}

func (a *API) listOrganizations(w http.ResponseWriter, r *http.Request) {
	organizations, err := a.db.ListOrganizations()
	if err != nil {
		render.Error(w, r, err)
		return
	}

	switch render.Format(r) {
	case render.FormatJSON:
		view := listOrganizationsView{}
		for _, org := range organizations {
			view.Organizations = append(view.Organizations, privateOrgView{
				ID:                 org.ExternalID,
				Name:               org.Name,
				CreatedAt:          org.FormatCreatedAt(),
				FirstProbeUpdateAt: org.FormatFirstProbeUpdateAt(),
				FeatureFlags:       org.FeatureFlags,
			})
		}
		render.JSON(w, http.StatusOK, view)
	default: // render.FormatHTML
		orgUsers := map[string]int{}
		for _, org := range organizations {
			us, err := a.db.ListOrganizationUsers(org.ExternalID)
			if err != nil {
				render.Error(w, r, err)
				return
			}
			orgUsers[org.ExternalID] = len(us)
		}

		b, err := a.templates.Bytes("list_organizations.html", map[string]interface{}{
			"Organizations":     organizations,
			"OrganizationUsers": orgUsers,
		})
		if err != nil {
			render.Error(w, r, err)
			return
		}
		if _, err := w.Write(b); err != nil {
			logrus.Warn("list organizations: %v", err)
		}
	}
}

func (a *API) setOrgFeatureFlags(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	orgExternalID, ok := vars["orgExternalID"]
	if !ok {
		render.Error(w, r, users.ErrNotFound)
		return
	}

	uniqueFlags := map[string]struct{}{}
	for _, f := range strings.Fields(r.FormValue("feature_flags")) {
		uniqueFlags[f] = struct{}{}
	}
	var sortedFlags []string
	for f := range uniqueFlags {
		sortedFlags = append(sortedFlags, f)
	}
	sort.Strings(sortedFlags)

	if err := a.db.SetFeatureFlags(orgExternalID, sortedFlags); err != nil {
		render.Error(w, r, err)
		return
	}
	redirectTo := r.FormValue("redirect_to")
	if redirectTo == "" {
		redirectTo = "/private/api/organizations"
	}
	http.Redirect(w, r, redirectTo, http.StatusFound)
}

func (a *API) marketingRefresh(w http.ResponseWriter, r *http.Request) {
	users, err := a.db.ListUsers()
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
	admin := r.URL.Query().Get("admin") == "true"
	if err := a.db.SetUserAdmin(userID, admin); err != nil {
		render.Error(w, r, err)
		return
	}
	redirectTo := r.FormValue("redirect_to")
	if redirectTo == "" {
		redirectTo = "/private/api/users"
	}
	http.Redirect(w, r, redirectTo, http.StatusFound)
}
