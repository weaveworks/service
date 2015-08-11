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
	sessions            SessionStore
	storage             Storage
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
	r.HandleFunc("/", Admin).Methods("GET")
	r.HandleFunc("/api/users/signup", Signup(directLogin)).Methods("POST")
	r.HandleFunc("/api/users/login", Login).Methods("GET")
	r.HandleFunc("/api/users/org/{orgName}", Authenticated(Org)).Methods("GET")
	r.HandleFunc("/api/users/org/{orgName}", Authenticated(RenameOrg)).Methods("PUT")
	r.HandleFunc("/api/users/org/{orgName}/users", Authenticated(ListOrganizationUsers)).Methods("GET")
	r.HandleFunc("/api/users/org/{orgName}/users", Authenticated(InviteUser)).Methods("POST")
	r.HandleFunc("/api/users/org/{orgName}/users/{userEmail}", Authenticated(DeleteUser)).Methods("DELETE")
	r.HandleFunc("/private/api/users/lookup/{orgName}", Lookup).Methods("GET")
	r.HandleFunc("/private/api/users", ListUnapprovedUsers).Methods("GET")
	r.HandleFunc("/private/api/users/{userID}/approve", ApproveUser).Methods("POST")
	return r
}

func Admin(w http.ResponseWriter, r *http.Request) {
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

func Signup(directLogin bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var view signupView
		if err := json.NewDecoder(r.Body).Decode(&view); err != nil {
			renderError(w, MalformedInputError(err))
			return
		}
		view.MailSent = false
		if view.Email == "" {
			renderError(w, ValidationErrorf("Email cannot be blank"))
			return
		}

		user, err := storage.FindUserByEmail(view.Email)
		if err == ErrNotFound {
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
			err = SendWelcomeEmail(user)
		} else {
			err = SendLoginEmail(user, token)
		}
		if err != nil {
			renderError(w, fmt.Errorf("Error sending login email: %s", err))
			return
		}
		view.MailSent = !directLogin

		renderJSON(w, http.StatusOK, view)
	}
}

func generateUserToken(storage Storage, user *User) (string, error) {
	token, err := user.GenerateToken()
	if err != nil {
		return "", err
	}
	if err := storage.SetUserToken(user.ID, token); err != nil {
		return "", err
	}
	return token, nil
}

func Login(w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("email")
	token := r.FormValue("token")
	switch {
	case email == "":
		renderError(w, ValidationErrorf("Email cannot be blank"))
		return
	case token == "":
		renderError(w, ValidationErrorf("Token cannot be blank"))
		return
	}

	tokenExpired := func(errs ...error) {
		for _, err := range errs {
			if err != nil {
				logrus.Error(err)
			}
		}
		renderError(w, ErrInvalidAuthenticationData)
		return
	}

	user, err := storage.FindUserByEmail(email)
	if err == ErrNotFound {
		user = &User{Token: "!"} // Will fail the token comparison
		err = nil
	}
	if renderError(w, err) {
		return
	}
	// We always do this so that the timing difference can't be used to infer a user's existence.
	if !user.CompareToken(token) {
		tokenExpired()
		return
	}
	if err := sessions.Set(w, user.ID); err != nil {
		tokenExpired(err)
		return
	}
	if err := storage.SetUserToken(user.ID, ""); err != nil {
		tokenExpired(err)
		return
	}
	renderJSON(w, http.StatusOK, map[string]interface{}{
		"email":            user.Email,
		"organizationName": user.Organization.Name,
	})
}

type lookupView struct {
	OrganizationID string `json:"organizationID"`
}

func Lookup(w http.ResponseWriter, r *http.Request) {
	orgName := mux.Vars(r)["orgName"]
	user, err := sessions.Get(r)
	if err == nil && user.Organization.Name == orgName {
		renderJSON(w, http.StatusOK, lookupView{OrganizationID: user.Organization.ID})
		return
	}

	if err != nil && err != ErrInvalidAuthenticationData {
		renderError(w, err)
		return
	}

	credentials, ok := parseAuthHeader(r.Header.Get("Authorization"))
	if ok && credentials.Realm == "Scope-Probe" {
		if token, ok := credentials.Params["token"]; ok {
			org, err := storage.FindOrganizationByProbeToken(token)
			if err == nil && org.Name == orgName {
				renderJSON(w, http.StatusOK, lookupView{OrganizationID: org.ID})
				return
			}

			if err != ErrInvalidAuthenticationData {
				renderError(w, err)
				return
			}
		}
	}

	w.WriteHeader(http.StatusUnauthorized)
}

type userView struct {
	ID        string    `json:"-"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"-"`
}

func (v userView) FormatCreatedAt() string {
	return v.CreatedAt.Format(time.Stamp)
}

// ListUnapprovedUsers lists users needing approval
func ListUnapprovedUsers(w http.ResponseWriter, r *http.Request) {
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

// ApproveUser approves a user by ID
func ApproveUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID, ok := vars["userID"]
	if !ok {
		renderError(w, ErrNotFound)
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
	if renderError(w, SendApprovedEmail(user, token)) {
		return
	}
	http.Redirect(w, r, "/private/api/users", http.StatusFound)
}

// Authenticated wraps a handlerfunc to make sure we have a logged in user, and
// they are accessing their own org.
func Authenticated(handler func(*User, http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, err := sessions.Get(r)
		if renderError(w, err) {
			return
		}

		orgName := mux.Vars(r)["orgName"]
		if orgName != user.Organization.Name {
			renderError(w, ErrInvalidAuthenticationData)
			return
		}

		handler(user, w, r)
	})
}

type orgView struct {
	User               string `json:"user"`
	Name               string `json:"name"`
	ProbeToken         string `json:"probeToken"`
	FirstProbeUpdateAt string `json:"firstProbeUpdateAt,omitempty"`
}

func Org(currentUser *User, w http.ResponseWriter, r *http.Request) {
	renderJSON(w, http.StatusOK, orgView{
		User:               currentUser.Email,
		Name:               currentUser.Organization.Name,
		ProbeToken:         currentUser.Organization.ProbeToken,
		FirstProbeUpdateAt: renderTime(currentUser.Organization.FirstProbeUpdateAt),
	})
}

func RenameOrg(currentUser *User, w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var view orgView
	err := json.NewDecoder(r.Body).Decode(&view)
	switch {
	case err != nil:
		renderError(w, MalformedInputError(err))
		return
	case view.Name == "":
		renderError(w, ValidationErrorf("Name cannot be blank"))
		return
	case !orgNameRegex.MatchString(view.Name):
		renderError(w, ValidationErrorf("Name can only contain letters, numbers, hyphen, and underscore"))
		return
	}

	if renderError(w, storage.RenameOrganization(mux.Vars(r)["orgName"], view.Name)) {
		return
	}
	w.WriteHeader(http.StatusOK)
}

func ListOrganizationUsers(currentUser *User, w http.ResponseWriter, r *http.Request) {
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

func InviteUser(currentUser *User, w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var view signupView
	if err := json.NewDecoder(r.Body).Decode(&view); err != nil {
		renderError(w, MalformedInputError(err))
		return
	}
	view.MailSent = false
	if view.Email == "" {
		renderError(w, ValidationErrorf("Email cannot be blank"))
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
	if err = SendInviteEmail(invitee, token); err != nil {
		renderError(w, fmt.Errorf("Error sending invite email: %s", err))
		return
	}
	view.MailSent = true

	renderJSON(w, http.StatusOK, view)
}

func DeleteUser(currentUser *User, w http.ResponseWriter, r *http.Request) {
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
