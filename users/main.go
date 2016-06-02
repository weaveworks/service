package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"regexp"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	"github.com/justinas/nosurf"
	"github.com/weaveworks/scope/common/middleware"

	"github.com/weaveworks/service/common/logging"
	"github.com/weaveworks/service/users/login"
	"github.com/weaveworks/service/users/pardot"
)

var (
	passwordHashingCost = 14
	orgNameRegex        = regexp.MustCompile(`\A[a-zA-Z0-9_-]+\z`)
	pardotClient        *pardot.Client
)

func main() {
	var (
		logLevel           = flag.String("log.level", "info", "Logging level to use: debug | info | warn | error")
		port               = flag.Int("port", 80, "port to listen on")
		domain             = flag.String("domain", "https://cloud.weave.works", "domain where scope service is runnning.")
		databaseURI        = flag.String("database-uri", "postgres://postgres@users-db.weave.local/users?sslmode=disable", "URI where the database can be found (for dev you can use memory://)")
		databaseMigrations = flag.String("database-migrations", "", "Path where the database migration files can be found")
		emailURI           = flag.String("email-uri", "", "uri of smtp server to send email through, of the format: smtp://username:password@hostname:port.  Either email-uri or sendgrid-api-key must be provided. For local development, you can set this to: log://, which will log all emails.")
		sessionSecret      = flag.String("session-secret", "XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX", "Secret used validate sessions")
		directLogin        = flag.Bool("direct-login", false, "Approve user and send login token in the signup response (DEV only)")

		pardotEmail    = flag.String("pardot-email", "", "Email of Pardot account.  If not supplied pardot integration will be disabled.")
		pardotPassword = flag.String("pardot-password", "", "Password of Pardot account.")
		pardotUserKey  = flag.String("pardot-userkey", "", "User key of Pardot account.")

		sendgridAPIKey   = flag.String("sendgrid-api-key", "", "Sendgrid API key.  Either email-uri or sendgrid-api-key must be provided.")
		emailFromAddress = flag.String("email-from-address", "Weave Cloud <support@weave.works>", "From address for emails.")
	)

	logins := login.NewProviders()
	logins.Register("github", login.NewGithubProvider())
	logins.Register("google", login.NewGoogleProvider())
	logins.Flags(flag.CommandLine)

	flag.Parse()

	if err := logging.Setup(*logLevel); err != nil {
		logrus.Fatalf("Error configuring logging: %v", err)
		return
	}

	if *pardotEmail != "" {
		pardotClient = pardot.NewClient(pardot.APIURL,
			*pardotEmail, *pardotPassword, *pardotUserKey)
		defer pardotClient.Stop()
	}

	rand.Seed(time.Now().UnixNano())

	setupLogging(*logLevel)

	templates := mustNewTemplateEngine()
	emailer := mustNewEmailer(*emailURI, *sendgridAPIKey, *emailFromAddress, templates, *domain)
	storage := mustNewDatabase(*databaseURI, *databaseMigrations)
	defer storage.Close()
	sessions := mustNewSessionStore(*sessionSecret, storage)

	logrus.Debug("Debug logging enabled")

	logrus.Infof("Listening on port %d", *port)
	http.Handle("/", newAPI(*directLogin, emailer, sessions, storage, logins, templates))
	http.Handle("/metrics", makePrometheusHandler())
	logrus.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *port), nil))
}

type api struct {
	directLogin bool
	sessions    sessionStore
	storage     database
	logins      *login.Providers
	templates   templateEngine
	emailer     emailer
	http.Handler
}

func newAPI(directLogin bool, emailer emailer, sessions sessionStore, storage database, logins *login.Providers, templates templateEngine) *api {
	a := &api{
		directLogin: directLogin,
		sessions:    sessions,
		storage:     storage,
		logins:      logins,
		templates:   templates,
		emailer:     emailer,
	}
	a.Handler = a.routes()
	return a
}

func (a *api) routes() http.Handler {
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
		{"api_users_org_orgName", "GET", "/api/users/org/{orgName}", a.authenticated(a.org)},
		{"api_users_org_orgName_rename", "PUT", "/api/users/org/{orgName}", a.authenticated(a.renameOrg)},

		// Used to list and manage organization access (invites)
		{"api_users_org_orgName_users", "GET", "/api/users/org/{orgName}/users", a.authenticated(a.listOrganizationUsers)},
		{"api_users_org_orgName_inviteUser", "POST", "/api/users/org/{orgName}/users", a.authenticated(a.inviteUser)},
		{"api_users_org_orgName_deleteUser", "DELETE", "/api/users/org/{orgName}/users/{userEmail}", a.authenticated(a.deleteUser)},

		// The users service client (i.e. our other services) use these to
		// authenticate the admin/user/probe.
		{"private_api_users_admin", "GET", "/private/api/users/admin", a.authenticated(a.lookupAdmin)},
		{"private_api_users_lookup_orgName", "GET", "/private/api/users/lookup/{orgName}", a.authenticated(a.lookupOrg)},
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
			Duration:     requestDuration,
		},
		middleware.Func(csrf),
	).Wrap(r)
}

func (a *api) admin(w http.ResponseWriter, r *http.Request) {
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

func (a *api) listLoginProviders(w http.ResponseWriter, r *http.Request) {
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
	renderJSON(w, http.StatusOK, view)
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
func (a *api) listAttachedLoginProviders(currentUser *user, w http.ResponseWriter, r *http.Request) {
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
	renderJSON(w, http.StatusOK, view)
}

type attachLoginProviderView struct {
	FirstLogin  bool `json:"firstLogin,omitempty"`
	UserCreated bool `json:"userCreated,omitempty"`
	Attach      bool `json:"attach,omitempty"`
}

func (a *api) attachLoginProvider(w http.ResponseWriter, r *http.Request) {
	view := attachLoginProviderView{}
	vars := mux.Vars(r)
	providerID := vars["provider"]
	provider, ok := a.logins.Get(providerID)
	if !ok {
		logrus.Errorf("Login provider not found: %q", providerID)
		renderError(w, r, errInvalidAuthenticationData)
		return
	}

	id, email, authSession, err := provider.Login(r)
	if err != nil {
		renderError(w, r, err)
		return
	}

	// Try and find an existing user to attach this login to.
	var u *user
	for _, f := range []func() (*user, error){
		func() (*user, error) {
			// If we have an existing session and an provider, we should use
			// that. This means that we'll associate the provider (if we have
			// one) with the logged in session.
			u, err := a.sessions.Get(r)
			switch err {
			case nil:
				view.Attach = true
			case errInvalidAuthenticationData:
				err = errNotFound
			}
			return u, err
		},
		func() (*user, error) {
			// If the user has already attached this provider, this is a no-op, so we
			// can just log them in with it.
			return a.storage.FindUserByLogin(providerID, id)
		},
		func() (*user, error) {
			// Match based on the user's email
			return a.storage.FindUserByEmail(email)
		},
	} {
		u, err = f()
		if err == nil {
			break
		} else if err != errNotFound {
			logrus.Error(err)
			renderError(w, r, errInvalidAuthenticationData)
			return
		}
	}

	if u == nil {
		// No matching user found, this must be a first-time-login with this
		// provider, so we'll create an account for them.
		view.UserCreated = true
		u, err = a.storage.CreateUser(email)
		if err != nil {
			logrus.Error(err)
			renderError(w, r, errInvalidAuthenticationData)
			return
		}
		pardotClient.UserCreated(u.Email, u.CreatedAt)
		u, err = a.storage.ApproveUser(u.ID)
		if err != nil {
			logrus.Error(err)
			renderError(w, r, errInvalidAuthenticationData)
			return
		}
	}

	if err := a.storage.AddLoginToUser(u.ID, providerID, id, authSession); err != nil {
		logrus.Error(err)
		renderError(w, r, errInvalidAuthenticationData)
		return
	}

	view.FirstLogin = u.FirstLoginAt.IsZero()

	if err := a.updateUserAtLogin(u); err != nil {
		renderError(w, r, err)
		return
	}

	if err := a.sessions.Set(w, u.ID, providerID); err != nil {
		renderError(w, r, errInvalidAuthenticationData)
		return
	}

	renderJSON(w, http.StatusOK, view)
}

func (a *api) detachLoginProvider(currentUser *user, w http.ResponseWriter, r *http.Request) {
	if err := a.storage.DetachLoginFromUser(currentUser.ID, mux.Vars(r)["provider"]); err != nil {
		renderError(w, r, err)
		return
	}
	renderJSON(w, http.StatusNoContent, nil)
}

type signupView struct {
	MailSent bool   `json:"mailSent"`
	Email    string `json:"email,omitempty"`
	Token    string `json:"token,omitempty"`
}

func (a *api) signup(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var view signupView
	if err := json.NewDecoder(r.Body).Decode(&view); err != nil {
		renderError(w, r, malformedInputError(err))
		return
	}
	view.MailSent = false
	if view.Email == "" {
		renderError(w, r, validationErrorf("Email cannot be blank"))
		return
	}

	user, err := a.storage.FindUserByEmail(view.Email)
	if err == errNotFound {
		user, err = a.storage.CreateUser(view.Email)
		// TODO(twilkie) I believe this is redundant, as Approve is also
		// called below
		if err == nil {
			pardotClient.UserCreated(user.Email, user.CreatedAt)
			user, err = a.storage.ApproveUser(user.ID)
		}
	}
	if err != nil {
		renderError(w, r, err)
		return
	}

	// We always do this so that the timing difference can't be used to infer a user's existence.
	token, err := generateUserToken(a.storage, user)
	if err != nil {
		renderError(w, r, fmt.Errorf("Error sending login email: %s", err))
		return
	}

	user, err = a.storage.ApproveUser(user.ID)
	if err != nil {
		renderError(w, r, fmt.Errorf("Error sending login email: %s", err))
		return
	}
	pardotClient.UserApproved(user.Email, user.ApprovedAt)

	if a.directLogin {
		view.Token = token
	}

	err = a.emailer.LoginEmail(user, token)
	if err != nil {
		renderError(w, r, fmt.Errorf("Error sending login email: %s", err))
		return
	}

	view.MailSent = true
	renderJSON(w, http.StatusOK, view)
}

func generateUserToken(storage database, user *user) (string, error) {
	token, err := generateToken()
	if err != nil {
		return "", err
	}
	if err := storage.SetUserToken(user.ID, token); err != nil {
		return "", err
	}
	return token, nil
}

type loginView struct {
	FirstLogin bool `json:"firstLogin,omitempty"`
}

func (a *api) login(w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("email")
	token := r.FormValue("token")
	switch {
	case email == "":
		renderError(w, r, validationErrorf("Email cannot be blank"))
		return
	case token == "":
		renderError(w, r, validationErrorf("Token cannot be blank"))
		return
	}

	u, err := a.storage.FindUserByEmail(email)
	if err == errNotFound {
		err = nil
	}
	if err != nil {
		logrus.Error(err)
		renderError(w, r, errInvalidAuthenticationData)
		return
	}

	// We always do this so that the timing difference can't be used to infer a user's existence.
	if !u.CompareToken(token) {
		renderError(w, r, errInvalidAuthenticationData)
		return
	}

	if err := a.storage.SetUserToken(u.ID, ""); err != nil {
		logrus.Error(err)
		renderError(w, r, errInvalidAuthenticationData)
		return
	}

	firstLogin := u.FirstLoginAt.IsZero()
	if err := a.updateUserAtLogin(u); err != nil {
		renderError(w, r, err)
		return
	}

	if err := a.sessions.Set(w, u.ID, ""); err != nil {
		renderError(w, r, errInvalidAuthenticationData)
		return
	}
	renderJSON(w, http.StatusOK, loginView{FirstLogin: firstLogin})
}

func (a *api) updateUserAtLogin(u *user) error {
	if u.FirstLoginAt.IsZero() {
		if err := a.storage.SetUserFirstLoginAt(u.ID); err != nil {
			return err
		}
	}
	if len(u.Organizations) == 0 {
		if _, err := a.storage.CreateOrganization(u.ID); err != nil {
			return err
		}
	}
	return nil
}

func (a *api) logout(_ *user, w http.ResponseWriter, r *http.Request) {
	a.sessions.Clear(w)
	renderJSON(w, http.StatusOK, map[string]interface{}{})
}

type publicLookupView struct {
	Email         string    `json:"email,omitempty"`
	Organizations []orgView `json:"organizations,omitempty"`
}

func (a *api) publicLookup(currentUser *user, w http.ResponseWriter, r *http.Request) {
	existing := []orgView{}
	for _, org := range currentUser.Organizations {
		existing = append(existing, orgView{
			Name:               org.Name,
			FirstProbeUpdateAt: renderTime(org.FirstProbeUpdateAt),
		})
	}

	renderJSON(w, http.StatusOK, publicLookupView{
		Email:         currentUser.Email,
		Organizations: existing,
	})
}

type lookupOrgView struct {
	OrganizationID string `json:"organizationID,omitempty"`
}

func (a *api) lookupOrg(currentUser *user, w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	orgName := vars["orgName"]
	for _, org := range currentUser.Organizations {
		if org.Name == orgName {
			renderJSON(w, http.StatusOK, lookupOrgView{OrganizationID: org.ID})
			return
		}
	}
	renderError(w, r, errNotFound)
}

type lookupAdminView struct {
	AdminID string `json:"adminID,omitempty"`
}

func (a *api) lookupAdmin(currentUser *user, w http.ResponseWriter, r *http.Request) {
	if currentUser.Admin {
		renderJSON(w, http.StatusOK, lookupAdminView{AdminID: currentUser.ID})
		return
	}
	w.WriteHeader(http.StatusUnauthorized)
}

func (a *api) lookupUsingToken(w http.ResponseWriter, r *http.Request) {
	credentials, ok := parseAuthHeader(r.Header.Get("Authorization"))
	if !ok || credentials.Realm != "Scope-Probe" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	token, ok := credentials.Params["token"]
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	org, err := a.storage.FindOrganizationByProbeToken(token)
	if err == nil {
		renderJSON(w, http.StatusOK, lookupOrgView{OrganizationID: org.ID})
		return
	}

	if err != errInvalidAuthenticationData {
		w.WriteHeader(http.StatusUnauthorized)
	} else {
		renderError(w, r, err)
	}
}

func (a *api) listUsers(w http.ResponseWriter, r *http.Request) {
	users, err := a.storage.ListUsers()
	if err != nil {
		renderError(w, r, err)
		return
	}
	b, err := a.templates.bytes("list_users.html", map[string]interface{}{
		"Users": users,
	})
	if err != nil {
		renderError(w, r, err)
		return
	}
	if _, err := w.Write(b); err != nil {
		logrus.Warn("list users: %v", err)
	}
}

func (a *api) pardotRefresh(w http.ResponseWriter, r *http.Request) {
	users, err := a.storage.ListUsers()
	if err != nil {
		renderError(w, r, err)
		return
	}

	for _, user := range users {
		// tell pardot about the users
		pardotClient.UserCreated(user.Email, user.CreatedAt)
		if !user.ApprovedAt.IsZero() {
			pardotClient.UserApproved(user.Email, user.ApprovedAt)
		}
	}
}

func (a *api) makeUserAdmin(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID, ok := vars["userID"]
	if !ok {
		renderError(w, r, errNotFound)
		return
	}
	admin := r.URL.Query().Get("admin") == "true"
	if err := a.storage.SetUserAdmin(userID, admin); err != nil {
		renderError(w, r, err)
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
func (a *api) authenticated(handler func(*user, http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, err := a.sessions.Get(r)
		if err != nil {
			renderError(w, r, err)
			return
		}

		// User actions always go through this endpoint because
		// app-mapper checks the authentication endpoint eevry time.
		// We use this to tell pardot about login activity.
		pardotClient.UserAccess(u.Email, time.Now())

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
	User               string `json:"user,omitempty"`
	Name               string `json:"name"`
	ProbeToken         string `json:"probeToken,omitempty"`
	FirstProbeUpdateAt string `json:"firstProbeUpdateAt,omitempty"`
}

func (a *api) org(currentUser *user, w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	orgName := vars["orgName"]
	for _, org := range currentUser.Organizations {
		if org.Name == orgName {
			renderJSON(w, http.StatusOK, orgView{
				User:               currentUser.Email,
				Name:               org.Name,
				ProbeToken:         org.ProbeToken,
				FirstProbeUpdateAt: renderTime(org.FirstProbeUpdateAt),
			})
			return
		}
	}
	renderError(w, r, errNotFound)
}

func (a *api) renameOrg(currentUser *user, w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var view orgView
	err := json.NewDecoder(r.Body).Decode(&view)
	switch {
	case err != nil:
		renderError(w, r, malformedInputError(err))
		return
	case view.Name == "":
		renderError(w, r, validationErrorf("Name cannot be blank"))
		return
	case !orgNameRegex.MatchString(view.Name):
		renderError(w, r, validationErrorf("Name can only contain letters, numbers, hyphen, and underscore"))
		return
	}

	if err := a.storage.RenameOrganization(mux.Vars(r)["orgName"], view.Name); err != nil {
		renderError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

type organizationUsersView struct {
	Users []organizationUserView `json:"users"`
}

type organizationUserView struct {
	Email string `json:"email"`
	Self  bool   `json:"self,omitempty"`
}

func (a *api) listOrganizationUsers(currentUser *user, w http.ResponseWriter, r *http.Request) {
	users, err := a.storage.ListOrganizationUsers(mux.Vars(r)["orgName"])
	if err != nil {
		renderError(w, r, err)
		return
	}
	view := organizationUsersView{}
	for _, u := range users {
		view.Users = append(view.Users, organizationUserView{
			Email: u.Email,
			Self:  u.ID == currentUser.ID,
		})
	}
	renderJSON(w, http.StatusOK, view)
}

func (a *api) inviteUser(currentUser *user, w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var view signupView
	if err := json.NewDecoder(r.Body).Decode(&view); err != nil {
		renderError(w, r, malformedInputError(err))
		return
	}
	view.MailSent = false
	if view.Email == "" {
		renderError(w, r, validationErrorf("Email cannot be blank"))
		return
	}

	orgName := mux.Vars(r)["orgName"]
	invitee, err := a.storage.InviteUser(view.Email, orgName)
	if err != nil {
		renderError(w, r, err)
		return
	}
	// Auto-approve all invited users
	invitee, err = a.storage.ApproveUser(invitee.ID)
	if err != nil {
		renderError(w, r, err)
		return
	}
	// We always do this so that the timing difference can't be used to infer a user's existence.
	token, err := generateUserToken(a.storage, invitee)
	if err != nil {
		renderError(w, r, fmt.Errorf("Error sending invite email: %s", err))
		return
	}
	if err = a.emailer.InviteEmail(invitee, orgName, token); err != nil {
		renderError(w, r, fmt.Errorf("Error sending invite email: %s", err))
		return
	}
	view.MailSent = true

	renderJSON(w, http.StatusOK, view)
}

func (a *api) deleteUser(currentUser *user, w http.ResponseWriter, r *http.Request) {
	if err := a.storage.DeleteUser(mux.Vars(r)["userEmail"]); err != nil {
		renderError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func renderTime(t time.Time) string {
	utc := t.UTC()
	if utc.IsZero() {
		return ""
	}
	return utc.Format(time.RFC3339)
}

func renderJSON(w http.ResponseWriter, status int, data interface{}) {
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		logrus.Error(err)
	}
}
