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
)

var (
	sessions            sessionStore
	storage             database
	passwordHashingCost = 14
	orgNameRegex        = regexp.MustCompile(`\A[a-zA-Z0-9_-]+\z`)
)

func main() {
	var (
		port          = flag.Int("port", 80, "port to listen on")
		databaseURI   = flag.String("database-uri", "postgres://postgres@users-db.weave.local/weave_development?sslmode=disable", "URI where the database can be found (for dev you can use memory://)")
		emailURI      = flag.String("email-uri", "smtp://smtp.weave.local:587", "uri of smtp server to send email through, of the format: smtp://username:password@hostname:port")
		logLevel      = flag.String("log-level", "info", "logging level (debug, info, warning, error)")
		sessionSecret = flag.String("session-secret", "XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX", "Secret used validate sessions")
		directLogin   = flag.Bool("direct-login", false, "Approve user and send login token in the signup response (DEV only)")
	)

	flag.Parse()

	rand.Seed(time.Now().UnixNano())

	setupLogging(*logLevel)
	setupEmail(*emailURI)
	setupStorage(*databaseURI)
	defer storage.Close()
	setupTemplates()
	setupSessions(*sessionSecret)
	logrus.Debug("Debug logging enabled")

	logrus.Infof("Listening on port %d", *port)
	logrus.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *port), handler(*directLogin)))
}

func handler(directLogin bool) http.Handler {
	return loggingMiddleware(routes(directLogin))
}

func routes(directLogin bool) http.Handler {
	r := mux.NewRouter()
	r.HandleFunc("/", admin).Methods("GET")
	r.HandleFunc("/api/users/signup", signup(directLogin)).Methods("POST")
	r.HandleFunc("/api/users/login", login).Methods("GET")
	r.HandleFunc("/api/users/logout", authenticated(logout)).Methods("GET")
	r.HandleFunc("/api/users/lookup", authenticated(publicLookup)).Methods("GET")
	r.HandleFunc("/api/users/org/{orgName}", authenticated(org)).Methods("GET")
	r.HandleFunc("/api/users/org/{orgName}", authenticated(renameOrg)).Methods("PUT")
	r.HandleFunc("/api/users/org/{orgName}/users", authenticated(listOrganizationUsers)).Methods("GET")
	r.HandleFunc("/api/users/org/{orgName}/users", authenticated(inviteUser)).Methods("POST")
	r.HandleFunc("/api/users/org/{orgName}/users/{userEmail}", authenticated(deleteUser)).Methods("DELETE")
	r.HandleFunc("/private/api/users/lookup/{orgName}", authenticated(lookupUsingCookie)).Methods("GET")
	r.HandleFunc("/private/api/users/lookup", lookupUsingToken).Methods("GET")
	r.HandleFunc("/private/api/users", listUnapprovedUsers).Methods("GET")
	r.HandleFunc("/private/api/users/{userID}/approve", approveUser).Methods("POST")
	return r
}

func admin(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/html")
	fmt.Fprintf(w, `
<html>
	<head><title>User service</title></head>
	<body>
		<h1>User service</h1>
		<ul>
			<li><a href="/private/api/users">Approve users</a></li>
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

func signup(directLogin bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var view signupView
		if err := json.NewDecoder(r.Body).Decode(&view); err != nil {
			renderError(w, malformedInputError(err))
			return
		}
		view.MailSent = false
		if view.Email == "" {
			renderError(w, validationErrorf("Email cannot be blank"))
			return
		}

		user, err := storage.FindUserByEmail(view.Email)
		if err == errNotFound {
			user, err = storage.CreateUser(view.Email)
		}
		if renderError(w, err) {
			return
		}
		// We always do this so that the timing difference can't be used to infer a user's existence.
		token, err := generateUserToken(storage, user)
		if err != nil {
			renderError(w, fmt.Errorf("Error sending login email: %s", err))
			return
		}
		if directLogin {
			// approve user, and return token
			_, err = storage.ApproveUser(user.ID)
			view.Token = token
		} else if user.ApprovedAt.IsZero() {
			err = sendWelcomeEmail(user)
		} else {
			err = sendLoginEmail(user, token)
		}
		if err != nil {
			renderError(w, fmt.Errorf("Error sending login email: %s", err))
			return
		}
		view.MailSent = !directLogin

		renderJSON(w, http.StatusOK, view)
	}
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

func login(w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("email")
	token := r.FormValue("token")
	switch {
	case email == "":
		renderError(w, validationErrorf("Email cannot be blank"))
		return
	case token == "":
		renderError(w, validationErrorf("Token cannot be blank"))
		return
	}

	tokenExpired := func(errs ...error) {
		for _, err := range errs {
			if err != nil {
				logrus.Error(err)
			}
		}
		renderError(w, errInvalidAuthenticationData)
		return
	}

	u, err := storage.FindUserByEmail(email)
	if err == errNotFound {
		u = &user{Token: "!"} // Will fail the token comparison
		err = nil
	}
	if renderError(w, err) {
		return
	}
	// We always do this so that the timing difference can't be used to infer a user's existence.
	if !u.CompareToken(token) {
		tokenExpired()
		return
	}
	if err := sessions.Set(w, u.ID); err != nil {
		tokenExpired(err)
		return
	}
	if err := storage.SetUserToken(u.ID, ""); err != nil {
		tokenExpired(err)
		return
	}
	renderJSON(w, http.StatusOK, map[string]interface{}{
		"email":            u.Email,
		"organizationName": u.Organization.Name,
	})
}

func logout(_ *user, w http.ResponseWriter, r *http.Request) {
	sessions.Clear(w)
	renderJSON(w, http.StatusOK, map[string]interface{}{})
}

type lookupView struct {
	OrganizationID     string `json:"organizationID,omitempty"`
	OrganizationName   string `json:"organizationName,omitempty"`
	FirstProbeUpdateAt string `json:"firstProbeUpdateAt,omitempty"`
}

func publicLookup(currentUser *user, w http.ResponseWriter, r *http.Request) {
	renderJSON(w, http.StatusOK, lookupView{
		OrganizationName:   currentUser.Organization.Name,
		FirstProbeUpdateAt: renderTime(currentUser.Organization.FirstProbeUpdateAt),
	})
}

func lookupUsingCookie(currentUser *user, w http.ResponseWriter, r *http.Request) {
	renderJSON(w, http.StatusOK, lookupView{OrganizationID: currentUser.Organization.ID})
}

func lookupUsingToken(w http.ResponseWriter, r *http.Request) {
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

	org, err := storage.FindOrganizationByProbeToken(token)
	if err == nil {
		renderJSON(w, http.StatusOK, lookupView{OrganizationID: org.ID})
		return
	}

	if err != errInvalidAuthenticationData {
		w.WriteHeader(http.StatusUnauthorized)
	} else {
		renderError(w, err)
	}
}

type userView struct {
	ID        string    `json:"-"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"-"`
}

func (v userView) FormatCreatedAt() string {
	return v.CreatedAt.Format(time.Stamp)
}

// listUnapprovedUsers lists users needing approval
func listUnapprovedUsers(w http.ResponseWriter, r *http.Request) {
	users, err := storage.ListUnapprovedUsers()
	if renderError(w, err) {
		return
	}
	userViews := []userView{}
	for _, u := range users {
		userViews = append(userViews, userView{u.ID, u.Email, u.CreatedAt})
	}
	b, err := templateBytes("list_users.html", userViews)
	if renderError(w, err) {
		return
	}
	w.Write(b)
}

// approveUser approves a user by ID
func approveUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID, ok := vars["userID"]
	if !ok {
		renderError(w, errNotFound)
		return
	}
	user, err := storage.ApproveUser(userID)
	if renderError(w, err) {
		return
	}
	token, err := generateUserToken(storage, user)
	if err != nil {
		renderError(w, fmt.Errorf("Error sending approved email: %s", err))
		return
	}
	if renderError(w, sendApprovedEmail(user, token)) {
		return
	}
	http.Redirect(w, r, "/private/api/users", http.StatusFound)
}

// authenticated wraps a handlerfunc to make sure we have a logged in user, and
// they are accessing their own org.
func authenticated(handler func(*user, http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, err := sessions.Get(r)
		if renderError(w, err) {
			return
		}

		orgName, hasOrgName := mux.Vars(r)["orgName"]
		if hasOrgName && orgName != u.Organization.Name {
			renderError(w, errInvalidAuthenticationData)
			return
		}

		handler(u, w, r)
	})
}

type orgView struct {
	User               string `json:"user"`
	Name               string `json:"name"`
	ProbeToken         string `json:"probeToken"`
	FirstProbeUpdateAt string `json:"firstProbeUpdateAt,omitempty"`
}

func org(currentUser *user, w http.ResponseWriter, r *http.Request) {
	renderJSON(w, http.StatusOK, orgView{
		User:               currentUser.Email,
		Name:               currentUser.Organization.Name,
		ProbeToken:         currentUser.Organization.ProbeToken,
		FirstProbeUpdateAt: renderTime(currentUser.Organization.FirstProbeUpdateAt),
	})
}

func renameOrg(currentUser *user, w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var view orgView
	err := json.NewDecoder(r.Body).Decode(&view)
	switch {
	case err != nil:
		renderError(w, malformedInputError(err))
		return
	case view.Name == "":
		renderError(w, validationErrorf("Name cannot be blank"))
		return
	case !orgNameRegex.MatchString(view.Name):
		renderError(w, validationErrorf("Name can only contain letters, numbers, hyphen, and underscore"))
		return
	}

	if renderError(w, storage.RenameOrganization(mux.Vars(r)["orgName"], view.Name)) {
		return
	}
	w.WriteHeader(http.StatusOK)
}

func listOrganizationUsers(currentUser *user, w http.ResponseWriter, r *http.Request) {
	users, err := storage.ListOrganizationUsers(mux.Vars(r)["orgName"])
	if renderError(w, err) {
		return
	}
	userViews := []userView{}
	for _, u := range users {
		userViews = append(userViews, userView{Email: u.Email})
	}
	renderJSON(w, http.StatusOK, userViews)
}

func inviteUser(currentUser *user, w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var view signupView
	if err := json.NewDecoder(r.Body).Decode(&view); err != nil {
		renderError(w, malformedInputError(err))
		return
	}
	view.MailSent = false
	if view.Email == "" {
		renderError(w, validationErrorf("Email cannot be blank"))
		return
	}

	invitee, err := storage.InviteUser(view.Email, mux.Vars(r)["orgName"])
	if renderError(w, err) {
		return
	}
	// We always do this so that the timing difference can't be used to infer a user's existence.
	token, err := generateUserToken(storage, invitee)
	if err != nil {
		renderError(w, fmt.Errorf("Error sending invite email: %s", err))
		return
	}
	if err = sendInviteEmail(invitee, token); err != nil {
		renderError(w, fmt.Errorf("Error sending invite email: %s", err))
		return
	}
	view.MailSent = true

	renderJSON(w, http.StatusOK, view)
}

func deleteUser(currentUser *user, w http.ResponseWriter, r *http.Request) {
	if renderError(w, storage.DeleteUser(mux.Vars(r)["userEmail"])) {
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
