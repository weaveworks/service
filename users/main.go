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
	"github.com/weaveworks/scope/common/middleware"

	"github.com/weaveworks/service/common/instrument"
	"github.com/weaveworks/service/users/pardot"
)

var (
	passwordHashingCost = 14
	orgNameRegex        = regexp.MustCompile(`\A[a-zA-Z0-9_-]+\z`)
	pardotClient        *pardot.Client
)

func main() {
	var (
		port               = flag.Int("port", 80, "port to listen on")
		domain             = flag.String("domain", "https://scope.weave.works", "domain where scope service is runnning.")
		databaseURI        = flag.String("database-uri", "postgres://postgres@users-db.weave.local/users?sslmode=disable", "URI where the database can be found (for dev you can use memory://)")
		databaseMigrations = flag.String("database-migrations", "", "Path where the database migration files can be found")
		emailURI           = flag.String("email-uri", "", "uri of smtp server to send email through, of the format: smtp://username:password@hostname:port.  Either email-uri or sendgrid-api-key must be provided. For local development, you can set this to: log://, which will log all emails.")
		logLevel           = flag.String("log-level", "info", "logging level (debug, info, warning, error)")
		sessionSecret      = flag.String("session-secret", "XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX", "Secret used validate sessions")
		directLogin        = flag.Bool("direct-login", false, "Approve user and send login token in the signup response (DEV only)")

		approvalRequired = flag.Bool("approval-required", true, "Do we want to gate users on approval.")
		pardotEmail      = flag.String("pardot-email", "", "Email of Pardot account.  If not supplied pardot integration will be disabled.")
		pardotPassword   = flag.String("pardot-password", "", "Password of Pardot account.")
		pardotUserKey    = flag.String("pardot-userkey", "", "User key of Pardot account.")

		sendgridAPIKey = flag.String("sendgrid-api-key", "", "Sendgrid API key.  Either email-uri or sendgrid-api-key must be provided.")
	)

	flag.Parse()

	if *pardotEmail != "" {
		pardotClient = pardot.NewClient(pardot.APIURL,
			*pardotEmail, *pardotPassword, *pardotUserKey)
		defer pardotClient.Stop()
	}

	rand.Seed(time.Now().UnixNano())

	setupLogging(*logLevel)

	templates := mustNewTemplateEngine()
	emailer := mustNewEmailer(*emailURI, *sendgridAPIKey, templates, *domain)
	storage := mustNewDatabase(*databaseURI, *databaseMigrations)
	defer storage.Close()
	sessions := mustNewSessionStore(*sessionSecret, storage)

	logrus.Debug("Debug logging enabled")

	logrus.Infof("Listening on port %d", *port)
	http.Handle("/", newAPI(*directLogin, *approvalRequired, emailer, sessions, storage, templates))
	http.Handle("/metrics", makePrometheusHandler())
	logrus.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *port), nil))
}

type api struct {
	directLogin      bool
	approvalRequired bool
	sessions         sessionStore
	storage          database
	templates        templateEngine
	emailer          emailer
	http.Handler
}

func newAPI(directLogin, approvalRequired bool, emailer emailer, sessions sessionStore, storage database, templates templateEngine) *api {
	a := &api{
		directLogin:      directLogin,
		approvalRequired: approvalRequired,
		sessions:         sessions,
		storage:          storage,
		templates:        templates,
		emailer:          emailer,
	}
	a.Handler = a.routes()
	return a
}

func (a *api) routes() http.Handler {
	r := mux.NewRouter()
	for _, route := range []struct {
		method, path string
		handler      http.HandlerFunc
	}{
		{"GET", "/loadgen", func(w http.ResponseWriter, _ *http.Request) { fmt.Fprintf(w, "OK") }},
		{"GET", "/", a.admin},
		{"POST", "/api/users/signup", a.signup},
		{"GET", "/api/users/login", a.login},
		{"GET", "/api/users/logout", a.authenticated(a.logout)},
		{"GET", "/api/users/lookup", a.authenticated(a.publicLookup)},
		{"GET", "/api/users/org/{orgName}", a.authenticated(a.org)},
		{"PUT", "/api/users/org/{orgName}", a.authenticated(a.renameOrg)},
		{"GET", "/api/users/org/{orgName}/users", a.authenticated(a.listOrganizationUsers)},
		{"POST", "/api/users/org/{orgName}/users", a.authenticated(a.inviteUser)},
		{"DELETE", "/api/users/org/{orgName}/users/{userEmail}", a.authenticated(a.deleteUser)},
		{"GET", "/private/api/users/lookup/{orgName}", a.authenticated(a.lookupUsingCookie)},
		{"GET", "/private/api/users/lookup", a.lookupUsingToken},
		{"GET", "/private/api/users", a.listUsers},
		{"GET", "/private/api/pardot", a.pardotRefresh},
		{"POST", "/private/api/users/{userID}/approve", a.approveUser},
	} {
		name := instrument.MakeLabelValue(route.path)
		r.Handle(route.path, route.handler).Name(name).Methods(route.method)
	}
	return middleware.Instrument{
		RouteMatcher: r,
		Duration:     requestDuration,
	}.Wrap(r)
}

func (a *api) admin(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/html")
	fmt.Fprintf(w, `
<html>
	<head><title>User service</title></head>
	<body>
		<h1>User service</h1>
		<ul>
			<li><a href="/private/api/users?approved=false">Approve users</a></li>
			<li><a href="/private/api/pardot">Sync User-Creation with Pardot</a></li>
		</ul>
	</body>
</html>
`)
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
		pardotClient.UserCreated(user.Email, user.CreatedAt)
	}
	if renderError(w, r, err) {
		return
	}

	// We always do this so that the timing difference can't be used to infer a user's existence.
	token, err := generateUserToken(a.storage, user)
	if err != nil {
		renderError(w, r, fmt.Errorf("Error sending login email: %s", err))
		return
	}
	if !a.approvalRequired || a.directLogin {
		_, err = a.storage.ApproveUser(user.ID)
	}
	if a.directLogin {
		view.Token = token
	} else if a.approvalRequired && user.ApprovedAt.IsZero() {
		err = a.emailer.WelcomeEmail(user)
	} else {
		err = a.emailer.LoginEmail(user, token)
	}
	if err != nil {
		renderError(w, r, fmt.Errorf("Error sending login email: %s", err))
		return
	}
	view.MailSent = !a.directLogin

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

	tokenExpired := func(errs ...error) {
		for _, err := range errs {
			if err != nil {
				logrus.Error(err)
			}
		}
		renderError(w, r, errInvalidAuthenticationData)
		return
	}

	u, err := a.storage.FindUserByEmail(email)
	if err == errNotFound {
		u = &user{Token: "!"} // Will fail the token comparison
		err = nil
	}
	if renderError(w, r, err) {
		return
	}
	// We always do this so that the timing difference can't be used to infer a user's existence.
	if !u.CompareToken(token) {
		tokenExpired()
		return
	}
	if u.FirstLoginAt.IsZero() {
		if err := a.storage.SetUserFirstLoginAt(u.ID); renderError(w, r, err) {
			return
		}
	}
	if err := a.sessions.Set(w, u.ID); err != nil {
		tokenExpired(err)
		return
	}
	if err := a.storage.SetUserToken(u.ID, ""); err != nil {
		tokenExpired(err)
		return
	}
	renderJSON(w, http.StatusOK, map[string]interface{}{
		"email":            u.Email,
		"organizationName": u.Organization.Name,
	})
}

func (a *api) logout(_ *user, w http.ResponseWriter, r *http.Request) {
	a.sessions.Clear(w)
	renderJSON(w, http.StatusOK, map[string]interface{}{})
}

type lookupView struct {
	OrganizationID     string `json:"organizationID,omitempty"`
	OrganizationName   string `json:"organizationName,omitempty"`
	FirstProbeUpdateAt string `json:"firstProbeUpdateAt,omitempty"`
	Admin              bool   `json:"admin,omitempty"`
}

func (a *api) publicLookup(currentUser *user, w http.ResponseWriter, r *http.Request) {
	renderJSON(w, http.StatusOK, lookupView{
		OrganizationName:   currentUser.Organization.Name,
		FirstProbeUpdateAt: renderTime(currentUser.Organization.FirstProbeUpdateAt),
	})
}

func (a *api) lookupUsingCookie(currentUser *user, w http.ResponseWriter, r *http.Request) {
	renderJSON(w, http.StatusOK, lookupView{OrganizationID: currentUser.Organization.ID, Admin: currentUser.Admin})
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
		renderJSON(w, http.StatusOK, lookupView{OrganizationID: org.ID})
		return
	}

	if err != errInvalidAuthenticationData {
		w.WriteHeader(http.StatusUnauthorized)
	} else {
		renderError(w, r, err)
	}
}

var listUsersFilters = filters{
	"approved": newUsersApprovedFilter,
}

func (a *api) listUsers(w http.ResponseWriter, r *http.Request) {
	users, err := a.storage.ListUsers(listUsersFilters.parse(r.URL.Query())...)
	if renderError(w, r, err) {
		return
	}
	b, err := a.templates.bytes("list_users.html", map[string]interface{}{
		"ShowingApproved":   r.URL.Query().Get("approved") == "true",
		"ShowingUnapproved": r.URL.Query().Get("approved") == "false",
		"Users":             users,
	})
	if renderError(w, r, err) {
		return
	}
	if _, err := w.Write(b); err != nil {
		logrus.Warn("list users: %v", err)
	}
}

func (a *api) pardotRefresh(w http.ResponseWriter, r *http.Request) {
	users, err := a.storage.ListUsers(listUsersFilters.parse(r.URL.Query())...)
	if renderError(w, r, err) {
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

func (a *api) approveUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID, ok := vars["userID"]
	if !ok {
		renderError(w, r, errNotFound)
		return
	}
	user, err := a.storage.ApproveUser(userID)
	if renderError(w, r, err) {
		return
	}
	token, err := generateUserToken(a.storage, user)
	if err != nil {
		renderError(w, r, fmt.Errorf("Error sending approved email: %s", err))
		return
	}
	pardotClient.UserApproved(user.Email, user.ApprovedAt)
	if renderError(w, r, a.emailer.ApprovedEmail(user, token)) {
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
		if renderError(w, r, err) {
			return
		}

		orgName, hasOrgName := mux.Vars(r)["orgName"]
		if hasOrgName && orgName != u.Organization.Name {
			renderError(w, r, errInvalidAuthenticationData)
			return
		}

		// User actions always go through this endpoint because
		// app-mapper checks the authentication endpoint eevry time.
		// We use this to tell pardot about login activity.
		pardotClient.UserAccess(u.Email, time.Now())

		handler(u, w, r)
	})
}

type orgView struct {
	User               string `json:"user"`
	Name               string `json:"name"`
	ProbeToken         string `json:"probeToken"`
	FirstProbeUpdateAt string `json:"firstProbeUpdateAt,omitempty"`
}

func (a *api) org(currentUser *user, w http.ResponseWriter, r *http.Request) {
	renderJSON(w, http.StatusOK, orgView{
		User:               currentUser.Email,
		Name:               currentUser.Organization.Name,
		ProbeToken:         currentUser.Organization.ProbeToken,
		FirstProbeUpdateAt: renderTime(currentUser.Organization.FirstProbeUpdateAt),
	})
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

	if renderError(w, r, a.storage.RenameOrganization(mux.Vars(r)["orgName"], view.Name)) {
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (a *api) listOrganizationUsers(currentUser *user, w http.ResponseWriter, r *http.Request) {
	users, err := a.storage.ListOrganizationUsers(mux.Vars(r)["orgName"])
	if renderError(w, r, err) {
		return
	}
	renderJSON(w, http.StatusOK, users)
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

	invitee, err := a.storage.InviteUser(view.Email, mux.Vars(r)["orgName"])
	if renderError(w, r, err) {
		return
	}
	// We always do this so that the timing difference can't be used to infer a user's existence.
	token, err := generateUserToken(a.storage, invitee)
	if err != nil {
		renderError(w, r, fmt.Errorf("Error sending invite email: %s", err))
		return
	}
	if err = a.emailer.InviteEmail(invitee, token); err != nil {
		renderError(w, r, fmt.Errorf("Error sending invite email: %s", err))
		return
	}
	view.MailSent = true

	renderJSON(w, http.StatusOK, view)
}

func (a *api) deleteUser(currentUser *user, w http.ResponseWriter, r *http.Request) {
	if renderError(w, r, a.storage.DeleteUser(mux.Vars(r)["userEmail"])) {
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
