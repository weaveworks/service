package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	"golang.org/x/net/context"

	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/client"
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
	users, err := a.db.ListUsers(r.Context())
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

func (a *API) listOrganizations(w http.ResponseWriter, r *http.Request) {
	organizations, err := a.db.ListOrganizations(r.Context())
	if err != nil {
		render.Error(w, r, err)
		return
	}

	orgUsers := map[string]int{}
	for _, org := range organizations {
		us, err := a.db.ListOrganizationUsers(r.Context(), org.ExternalID)
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
	case "DenyUIFeatures":
		deny := value == "on"
		err = a.db.SetOrganizationDenyUIFeatures(r.Context(), orgExternalID, deny)
	case "DenyTokenAuth":
		deny := value == "on"
		err = a.db.SetOrganizationDenyTokenAuth(r.Context(), orgExternalID, deny)
	case "FeatureFlags":
		uniqueFlags := map[string]struct{}{}
		for _, f := range strings.Fields(value) {
			uniqueFlags[f] = struct{}{}
		}
		var sortedFlags []string
		for f := range uniqueFlags {
			sortedFlags = append(sortedFlags, f)
		}
		sort.Strings(sortedFlags)

		err = a.db.SetFeatureFlags(r.Context(), orgExternalID, sortedFlags)
	default:
		err = users.ValidationErrorf("Invalid field %v", field)
		return
	}

	if err != nil {
		render.Error(w, r, err)
		return
	}

	http.Redirect(w, r, "/admin/users/organizations", http.StatusFound)
}

func (a *API) marketingRefresh(w http.ResponseWriter, r *http.Request) {
	users, err := a.db.ListUsers(r.Context())
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
	logrus.Info(r)
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
	if err := a.sessions.Set(w, u.ID); err != nil {
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
