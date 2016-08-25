package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	"github.com/justinas/nosurf"
	"github.com/weaveworks/scope/common/middleware"

	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/emailer"
	"github.com/weaveworks/service/users/login"
	"github.com/weaveworks/service/users/pardot"
	"github.com/weaveworks/service/users/render"
	"github.com/weaveworks/service/users/sessions"
	"github.com/weaveworks/service/users/storage"
	"github.com/weaveworks/service/users/templates"
	"github.com/weaveworks/service/users/tokens"
)

// API implements the users api.
type API struct {
	directLogin       bool
	sessions          sessions.Store
	db                storage.Database
	logins            *login.Providers
	templates         templates.Engine
	emailer           emailer.Emailer
	pardotClient      *pardot.Client
	forceFeatureFlags []string
	http.Handler
}

// New creates a new API
func New(
	directLogin bool,
	emailer emailer.Emailer,
	sessions sessions.Store,
	db storage.Database,
	logins *login.Providers,
	templates templates.Engine,
	pardotClient *pardot.Client,
	forceFeatureFlags []string,
) *API {
	a := &API{
		directLogin:       directLogin,
		sessions:          sessions,
		db:                db,
		logins:            logins,
		templates:         templates,
		emailer:           emailer,
		pardotClient:      pardotClient,
		forceFeatureFlags: forceFeatureFlags,
	}
	a.Handler = a.routes()
	return a
}

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
			<li><a href="private/api/pardot">Sync User-Creation with Pardot</a></li>
		</ul>
	</body>
</html>
`)
}

type loginProvidersView struct {
	Logins []loginProviderView `json:"logins"`
}

type loginProviderView struct {
	ID   string     `json:"id"`
	Name string     `json:"name"` // Human-readable name of this provider
	Link login.Link `json:"link"` // HTML Attributes for the link to start this provider flow
}

func (a *API) listLoginProviders(w http.ResponseWriter, r *http.Request) {
	view := loginProvidersView{}
	a.logins.ForEach(func(id string, p login.Provider) {
		v := loginProviderView{
			ID:   id,
			Name: p.Name(),
		}
		if link, ok := p.Link(r); ok {
			v.Link = link
		}
		view.Logins = append(view.Logins, v)
	})
	render.JSON(w, http.StatusOK, view)
}

type attachedLoginProvidersView struct {
	Logins []attachedLoginProviderView `json:"logins"`
}

type attachedLoginProviderView struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	LoginID  string `json:"loginID,omitempty"`
	Username string `json:"username,omitempty"`
}

// List all the login providers currently attached to the current user. Used by
// the /account page to determine which login providers are currently attached.
func (a *API) listAttachedLoginProviders(currentUser *users.User, w http.ResponseWriter, r *http.Request) {
	view := attachedLoginProvidersView{}
	for _, l := range currentUser.Logins {
		p, ok := a.logins.Get(l.Provider)
		if !ok {
			continue
		}

		v := attachedLoginProviderView{
			ID:      l.Provider,
			Name:    p.Name(),
			LoginID: l.ProviderID,
		}
		v.Username, _ = p.Username(l.Session)
		view.Logins = append(view.Logins, v)
	}
	render.JSON(w, http.StatusOK, view)
}

type attachLoginProviderView struct {
	FirstLogin  bool `json:"firstLogin,omitempty"`
	UserCreated bool `json:"userCreated,omitempty"`
	Attach      bool `json:"attach,omitempty"`
}

func (a *API) attachLoginProvider(w http.ResponseWriter, r *http.Request) {
	view := attachLoginProviderView{}
	vars := mux.Vars(r)
	providerID := vars["provider"]
	provider, ok := a.logins.Get(providerID)
	if !ok {
		logrus.Errorf("Login provider not found: %q", providerID)
		render.Error(w, r, users.ErrInvalidAuthenticationData)
		return
	}

	id, email, authSession, err := provider.Login(r)
	if err != nil {
		render.Error(w, r, err)
		return
	}

	// Try and find an existing user to attach this login to.
	var u *users.User
	for _, f := range []func() (*users.User, error){
		func() (*users.User, error) {
			// If we have an existing session and an provider, we should use
			// that. This means that we'll associate the provider (if we have
			// one) with the logged in session.
			u, err := a.sessions.Get(r)
			switch err {
			case nil:
				view.Attach = true
			case users.ErrInvalidAuthenticationData:
				err = users.ErrNotFound
			}
			return u, err
		},
		func() (*users.User, error) {
			// If the user has already attached this provider, this is a no-op, so we
			// can just log them in with it.
			return a.db.FindUserByLogin(providerID, id)
		},
		func() (*users.User, error) {
			// Match based on the user's email
			return a.db.FindUserByEmail(email)
		},
	} {
		u, err = f()
		if err == nil {
			break
		} else if err != users.ErrNotFound {
			logrus.Error(err)
			render.Error(w, r, users.ErrInvalidAuthenticationData)
			return
		}
	}

	if u == nil {
		// No matching user found, this must be a first-time-login with this
		// provider, so we'll create an account for them.
		view.UserCreated = true
		u, err = a.db.CreateUser(email)
		if err != nil {
			logrus.Error(err)
			render.Error(w, r, users.ErrInvalidAuthenticationData)
			return
		}
		a.pardotClient.UserCreated(u.Email, u.CreatedAt)
		u, err = a.db.ApproveUser(u.ID)
		if err != nil {
			logrus.Error(err)
			render.Error(w, r, users.ErrInvalidAuthenticationData)
			return
		}
	}

	if err := a.db.AddLoginToUser(u.ID, providerID, id, authSession); err != nil {
		logrus.Error(err)
		render.Error(w, r, users.ErrInvalidAuthenticationData)
		return
	}

	view.FirstLogin = u.FirstLoginAt.IsZero()

	if err := a.updateUserAtLogin(u); err != nil {
		render.Error(w, r, err)
		return
	}

	if err := a.sessions.Set(w, u.ID, providerID); err != nil {
		render.Error(w, r, users.ErrInvalidAuthenticationData)
		return
	}

	render.JSON(w, http.StatusOK, view)
}

func (a *API) detachLoginProvider(currentUser *users.User, w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	providerID := vars["provider"]
	provider, ok := a.logins.Get(providerID)
	if !ok {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	for _, login := range currentUser.Logins {
		if login.Provider != providerID {
			continue
		}
		if err := provider.Logout(login.Session); err != nil {
			render.Error(w, r, err)
			return
		}
	}

	if err := a.db.DetachLoginFromUser(currentUser.ID, providerID); err != nil {
		render.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type signupView struct {
	MailSent bool   `json:"mailSent"`
	Email    string `json:"email,omitempty"`
	Token    string `json:"token,omitempty"`
}

func (a *API) signup(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var view signupView
	if err := json.NewDecoder(r.Body).Decode(&view); err != nil {
		render.Error(w, r, users.MalformedInputError(err))
		return
	}
	view.MailSent = false
	if view.Email == "" {
		render.Error(w, r, users.ValidationErrorf("Email cannot be blank"))
		return
	}

	user, err := a.db.FindUserByEmail(view.Email)
	if err == users.ErrNotFound {
		user, err = a.db.CreateUser(view.Email)
		// TODO(twilkie) I believe this is redundant, as Approve is also
		// called below
		if err == nil {
			a.pardotClient.UserCreated(user.Email, user.CreatedAt)
			user, err = a.db.ApproveUser(user.ID)
		}
	}
	if err != nil {
		render.Error(w, r, err)
		return
	}

	// We always do this so that the timing difference can't be used to infer a user's existence.
	token, err := generateUserToken(a.db, user)
	if err != nil {
		render.Error(w, r, fmt.Errorf("Error sending login email: %s", err))
		return
	}

	user, err = a.db.ApproveUser(user.ID)
	if err != nil {
		render.Error(w, r, fmt.Errorf("Error sending login email: %s", err))
		return
	}
	a.pardotClient.UserApproved(user.Email, user.ApprovedAt)

	if a.directLogin {
		view.Token = token
	}

	err = a.emailer.LoginEmail(user, token)
	if err != nil {
		render.Error(w, r, fmt.Errorf("Error sending login email: %s", err))
		return
	}

	view.MailSent = true
	render.JSON(w, http.StatusOK, view)
}

func generateUserToken(db storage.Database, user *users.User) (string, error) {
	token, err := tokens.Generate()
	if err != nil {
		return "", err
	}
	if err := db.SetUserToken(user.ID, token); err != nil {
		return "", err
	}
	return token, nil
}

type loginView struct {
	FirstLogin bool `json:"firstLogin,omitempty"`
}

func (a *API) login(w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("email")
	token := r.FormValue("token")
	switch {
	case email == "":
		render.Error(w, r, users.ValidationErrorf("Email cannot be blank"))
		return
	case token == "":
		render.Error(w, r, users.ValidationErrorf("Token cannot be blank"))
		return
	}

	u, err := a.db.FindUserByEmail(email)
	if err == users.ErrNotFound {
		err = nil
	}
	if err != nil {
		logrus.Error(err)
		render.Error(w, r, users.ErrInvalidAuthenticationData)
		return
	}

	// We always do this so that the timing difference can't be used to infer a user's existence.
	if !u.CompareToken(token) {
		render.Error(w, r, users.ErrInvalidAuthenticationData)
		return
	}

	if err := a.db.SetUserToken(u.ID, ""); err != nil {
		logrus.Error(err)
		render.Error(w, r, users.ErrInvalidAuthenticationData)
		return
	}

	firstLogin := u.FirstLoginAt.IsZero()
	if err := a.updateUserAtLogin(u); err != nil {
		render.Error(w, r, err)
		return
	}

	if err := a.sessions.Set(w, u.ID, ""); err != nil {
		render.Error(w, r, users.ErrInvalidAuthenticationData)
		return
	}
	render.JSON(w, http.StatusOK, loginView{FirstLogin: firstLogin})
}

func (a *API) updateUserAtLogin(u *users.User) error {
	if u.FirstLoginAt.IsZero() {
		if err := a.db.SetUserFirstLoginAt(u.ID); err != nil {
			return err
		}
	}
	return nil
}

func (a *API) logout(_ *users.User, w http.ResponseWriter, r *http.Request) {
	a.sessions.Clear(w)
	render.JSON(w, http.StatusOK, map[string]interface{}{})
}

type publicLookupView struct {
	Email         string    `json:"email,omitempty"`
	Organizations []orgView `json:"organizations,omitempty"`
}

func (a *API) publicLookup(currentUser *users.User, w http.ResponseWriter, r *http.Request) {
	existing := []orgView{}
	for _, org := range currentUser.Organizations {
		existing = append(existing, orgView{
			ExternalID:         org.ExternalID,
			Name:               org.Name,
			FirstProbeUpdateAt: render.Time(org.FirstProbeUpdateAt),
			FeatureFlags:       append(org.FeatureFlags, a.forceFeatureFlags...),
		})
	}

	render.JSON(w, http.StatusOK, publicLookupView{
		Email:         currentUser.Email,
		Organizations: existing,
	})
}

type lookupOrgView struct {
	OrganizationID string `json:"organizationID,omitempty"`
	UserID         string `json:"userID,omitempty"`
}

func (a *API) lookupOrg(currentUser *users.User, w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	orgExternalID := vars["orgExternalID"]
	for _, org := range currentUser.Organizations {
		if strings.ToLower(org.ExternalID) == strings.ToLower(orgExternalID) {
			render.JSON(w, http.StatusOK, lookupOrgView{
				OrganizationID: org.ID,
				UserID:         currentUser.ID,
			})
			return
		}
	}
	render.Error(w, r, users.ErrNotFound)
}

type lookupAdminView struct {
	AdminID string `json:"adminID,omitempty"`
}

func (a *API) lookupAdmin(currentUser *users.User, w http.ResponseWriter, r *http.Request) {
	if currentUser.Admin {
		render.JSON(w, http.StatusOK, lookupAdminView{AdminID: currentUser.ID})
		return
	}
	w.WriteHeader(http.StatusUnauthorized)
}

func (a *API) lookupUsingToken(w http.ResponseWriter, r *http.Request) {
	credentials, ok := ParseAuthHeader(r.Header.Get("Authorization"))
	if !ok || credentials.Realm != "Scope-Probe" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	token, ok := credentials.Params["token"]
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	org, err := a.db.FindOrganizationByProbeToken(token)
	if err == nil {
		render.JSON(w, http.StatusOK, lookupOrgView{OrganizationID: org.ID})
		return
	}

	if err != users.ErrInvalidAuthenticationData {
		w.WriteHeader(http.StatusUnauthorized)
	} else {
		render.Error(w, r, err)
	}
}

func (a *API) listUsers(w http.ResponseWriter, r *http.Request) {
	users, err := a.db.ListUsers()
	if err != nil {
		render.Error(w, r, err)
		return
	}
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

func (a *API) pardotRefresh(w http.ResponseWriter, r *http.Request) {
	users, err := a.db.ListUsers()
	if err != nil {
		render.Error(w, r, err)
		return
	}

	for _, user := range users {
		// tell pardot about the users
		a.pardotClient.UserCreated(user.Email, user.CreatedAt)
		if !user.ApprovedAt.IsZero() {
			a.pardotClient.UserApproved(user.Email, user.ApprovedAt)
		}
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

// authenticated wraps a handlerfunc to make sure we have a logged in user, and
// they are accessing their own org.
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

type orgView struct {
	User               string   `json:"user,omitempty"`
	ExternalID         string   `json:"id"`
	Name               string   `json:"name"`
	ProbeToken         string   `json:"probeToken,omitempty"`
	FirstProbeUpdateAt string   `json:"firstProbeUpdateAt,omitempty"`
	FeatureFlags       []string `json:"featureFlags,omitempty"`
}

func (a *API) org(currentUser *users.User, w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	orgExternalID := vars["orgExternalID"]
	for _, org := range currentUser.Organizations {
		if strings.ToLower(org.ExternalID) == strings.ToLower(orgExternalID) {
			render.JSON(w, http.StatusOK, orgView{
				User:               currentUser.Email,
				ExternalID:         org.ExternalID,
				Name:               org.Name,
				ProbeToken:         org.ProbeToken,
				FirstProbeUpdateAt: render.Time(org.FirstProbeUpdateAt),
				FeatureFlags:       append(org.FeatureFlags, a.forceFeatureFlags...),
			})
			return
		}
	}
	if exists, err := a.db.OrganizationExists(orgExternalID); err != nil {
		render.Error(w, r, err)
		return
	} else if exists {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	render.Error(w, r, users.ErrNotFound)
}

func (a *API) generateOrgExternalID(currentUser *users.User, w http.ResponseWriter, r *http.Request) {
	externalID, err := a.db.GenerateOrganizationExternalID()
	if err != nil {
		render.Error(w, r, err)
		return
	}
	render.JSON(w, http.StatusOK, orgView{Name: externalID, ExternalID: externalID})
}

func (a *API) createOrg(currentUser *users.User, w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var view orgView
	err := json.NewDecoder(r.Body).Decode(&view)
	switch {
	case err != nil:
		render.Error(w, r, users.MalformedInputError(err))
		return
	}

	if _, err := a.db.CreateOrganization(currentUser.ID, view.ExternalID, view.Name); err != nil {
		render.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (a *API) updateOrg(currentUser *users.User, w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var view orgView
	err := json.NewDecoder(r.Body).Decode(&view)
	switch {
	case err != nil:
		render.Error(w, r, users.MalformedInputError(err))
		return
	case view.ExternalID != "":
		render.Error(w, r, users.ValidationErrorf("ID cannot be changed"))
		return
	}

	orgExternalID := mux.Vars(r)["orgExternalID"]
	if err := a.userCanAccessOrg(currentUser, orgExternalID); err != nil {
		render.Error(w, r, err)
		return
	}
	if err := a.db.RenameOrganization(orgExternalID, view.Name); err != nil {
		render.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) deleteOrg(currentUser *users.User, w http.ResponseWriter, r *http.Request) {
	orgExternalID := mux.Vars(r)["orgExternalID"]
	if !currentUser.HasOrganization(orgExternalID) {
		if exists, err := a.db.OrganizationExists(orgExternalID); err != nil {
			render.Error(w, r, err)
			return
		} else if exists {
			w.WriteHeader(http.StatusForbidden)
			return
		}
	}
	if err := a.db.DeleteOrganization(orgExternalID); err != nil {
		render.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type organizationUsersView struct {
	Users []organizationUserView `json:"users"`
}

type organizationUserView struct {
	Email string `json:"email"`
	Self  bool   `json:"self,omitempty"`
}

func (a *API) listOrganizationUsers(currentUser *users.User, w http.ResponseWriter, r *http.Request) {
	orgExternalID := mux.Vars(r)["orgExternalID"]
	if err := a.userCanAccessOrg(currentUser, orgExternalID); err != nil {
		render.Error(w, r, err)
		return
	}

	users, err := a.db.ListOrganizationUsers(orgExternalID)
	if err != nil {
		render.Error(w, r, err)
		return
	}
	view := organizationUsersView{}
	for _, u := range users {
		view.Users = append(view.Users, organizationUserView{
			Email: u.Email,
			Self:  u.ID == currentUser.ID,
		})
	}
	render.JSON(w, http.StatusOK, view)
}

func (a *API) inviteUser(currentUser *users.User, w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var view signupView
	if err := json.NewDecoder(r.Body).Decode(&view); err != nil {
		render.Error(w, r, users.MalformedInputError(err))
		return
	}
	view.MailSent = false
	if view.Email == "" {
		render.Error(w, r, users.ValidationErrorf("Email cannot be blank"))
		return
	}

	orgExternalID := mux.Vars(r)["orgExternalID"]
	if err := a.userCanAccessOrg(currentUser, orgExternalID); err != nil {
		render.Error(w, r, err)
		return
	}

	invitee, created, err := a.db.InviteUser(view.Email, orgExternalID)
	if err != nil {
		render.Error(w, r, err)
		return
	}
	// Auto-approve all invited users
	invitee, err = a.db.ApproveUser(invitee.ID)
	if err != nil {
		render.Error(w, r, err)
		return
	}
	// We always do this so that the timing difference can't be used to infer a user's existence.
	token, err := generateUserToken(a.db, invitee)
	if err != nil {
		render.Error(w, r, fmt.Errorf("Error sending invite email: %s", err))
		return
	}
	orgName, err := a.db.GetOrganizationName(orgExternalID)
	if err != nil {
		render.Error(w, r, fmt.Errorf("Error getting organization name: %s", err))
	}

	if created {
		err = a.emailer.InviteEmail(currentUser, invitee, orgExternalID, orgName, token)
	} else {
		err = a.emailer.GrantAccessEmail(currentUser, invitee, orgExternalID, orgName)
	}
	if err != nil {
		render.Error(w, r, fmt.Errorf("Error sending invite email: %s", err))
		return
	}
	view.MailSent = true

	render.JSON(w, http.StatusOK, view)
}

func (a *API) deleteUser(currentUser *users.User, w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	orgExternalID := vars["orgExternalID"]
	userEmail := vars["userEmail"]
	if err := a.userCanAccessOrg(currentUser, orgExternalID); err != nil {
		render.Error(w, r, err)
		return
	}

	if err := a.db.RemoveUserFromOrganization(orgExternalID, userEmail); err != nil {
		render.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) userCanAccessOrg(currentUser *users.User, orgExternalID string) error {
	if !currentUser.HasOrganization(orgExternalID) {
		if exists, err := a.db.OrganizationExists(orgExternalID); err != nil {
			return err
		} else if exists {
			return users.ErrForbidden
		}
		return users.ErrNotFound
	}
	return nil
}
