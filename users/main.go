package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
)

var (
	sessions            SessionStore
	storage             Storage
	passwordHashingCost = 14
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
	r.HandleFunc("/private/api/users/lookup/{orgName}", Lookup).Methods("GET")
	r.HandleFunc("/private/api/users", ListUsers).Methods("GET")
	r.HandleFunc("/private/api/users/{userID}/approve", ApproveUser).Methods("POST")
	r.HandleFunc("/api/users/org/{orgName}", Org).Methods("GET")
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
			renderError(w, http.StatusBadRequest, err)
			return
		}
		view.MailSent = false
		if view.Email == "" {
			renderError(w, http.StatusBadRequest, fmt.Errorf("Email cannot be blank"))
			return
		}

		user, err := storage.FindUserByEmail(view.Email)
		if err == ErrNotFound {
			user, err = storage.CreateUser(view.Email)
		}
		if err != nil {
			internalServerError(w, err)
			return
		}
		// We always do this so that the timing difference can't be used to infer a user's existence.
		token, err := generateUserToken(storage, user)
		if err != nil {
			logrus.Error(err)
			renderError(w, http.StatusInternalServerError, fmt.Errorf("Error sending login email"))
			return
		}
		if directLogin {
			// approve user, and return token
			if user.Organization == nil {
				_, err = storage.ApproveUser(user.ID)
			}
			view.Token = token
		} else if user.ApprovedAt.IsZero() {
			err = SendWelcomeEmail(user)
		} else {
			err = SendLoginEmail(user, token)
		}
		if err != nil {
			logrus.Error(err)
			renderError(w, http.StatusInternalServerError, fmt.Errorf("Error sending login email"))
			return
		} else {
			view.MailSent = !directLogin
		}

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
	errs := Errors{}
	if email == "" {
		errs = append(errs, fmt.Errorf("Email cannot be blank"))
	}
	if token == "" {
		errs = append(errs, fmt.Errorf("Token cannot be blank"))
	}
	if len(errs) > 0 {
		renderError(w, http.StatusBadRequest, errs)
		return
	}

	tokenExpired := func(errs ...error) {
		for _, err := range errs {
			if err != nil {
				logrus.Error(err)
			}
		}
		renderError(w, http.StatusUnauthorized, ErrInvalidAuthenticationData)
		return
	}

	user, err := storage.FindUserByEmail(email)
	if err == ErrNotFound {
		user = &User{Token: "!"} // Will fail the token comparison
		err = nil
	}
	if err != nil {
		internalServerError(w, err)
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
		internalServerError(w, err)
		return
	}

	authHeader := r.Header.Get("Authorization")
	if fields := strings.Fields(authHeader); len(fields) == 2 && fields[0] == "Scope-Probe" {
		org, err := storage.FindOrganizationByProbeToken(fields[1])
		if err == nil && org.Name == orgName {
			renderJSON(w, http.StatusOK, lookupView{OrganizationID: org.ID})
			return
		}

		if err != ErrInvalidAuthenticationData {
			internalServerError(w, err)
			return
		}
	}

	w.WriteHeader(http.StatusUnauthorized)
}

type userView struct {
	ID        string
	Email     string
	CreatedAt time.Time
}

func (v userView) FormatCreatedAt() string {
	return v.CreatedAt.Format(time.Stamp)
}

// List users needing approval
func ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := storage.ListUnapprovedUsers()
	if err != nil {
		internalServerError(w, err)
		return
	}
	userViews := []userView{}
	for _, u := range users {
		userViews = append(userViews, userView{u.ID, u.Email, u.CreatedAt})
	}
	b, err := templateBytes("list_users.html", userViews)
	if err != nil {
		internalServerError(w, err)
		return
	}
	w.Write(b)
}

// Approve a user by ID
func ApproveUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID, ok := vars["userID"]
	if !ok {
		renderError(w, http.StatusNotFound, fmt.Errorf("Not Found"))
		return
	}
	user, err := storage.ApproveUser(userID)
	switch {
	case err == ErrNotFound:
		renderError(w, http.StatusNotFound, fmt.Errorf("Not Found"))
		return
	case err != nil:
		internalServerError(w, err)
		return
	}
	token, err := generateUserToken(storage, user)
	if err != nil {
		logrus.Error(err)
		renderError(w, http.StatusInternalServerError, fmt.Errorf("Error sending approved email"))
		return
	}
	if err := SendApprovedEmail(user, token); err != nil {
		internalServerError(w, err)
		return
	}
	http.Redirect(w, r, "/private/api/users", http.StatusFound)
}

type orgView struct {
	User               string `json:"user"`
	Name               string `json:"name"`
	ProbeToken         string `json:"probeToken"`
	FirstProbeUpdateAt string `json:"firstProbeUpdateAt,omitempty"`
}

func Org(w http.ResponseWriter, r *http.Request) {
	user, err := sessions.Get(r)
	if err != nil {
		if err == ErrInvalidAuthenticationData {
			renderError(w, http.StatusUnauthorized, err)
		} else {
			internalServerError(w, err)
		}
		return
	}
	renderJSON(w, http.StatusOK, orgView{
		User:               user.Email,
		Name:               user.Organization.Name,
		ProbeToken:         user.Organization.ProbeToken,
		FirstProbeUpdateAt: renderTime(user.Organization.FirstProbeUpdateAt),
	})
}

func renderTime(t time.Time) string {
	utc := t.UTC()
	if utc.IsZero() {
		return ""
	}
	return utc.Format(time.RFC3339)
}

func internalServerError(w http.ResponseWriter, err error) {
	logrus.Error(err)
	http.Error(w, `{"errors":[{"message":"An internal server error occurred"}]}`, http.StatusInternalServerError)
}

func renderJSON(w http.ResponseWriter, status int, data interface{}) {
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		logrus.Error(err)
	}
}

func renderError(w http.ResponseWriter, status int, err error) bool {
	if err == nil {
		return false
	}

	var view errorsView
	switch err := err.(type) {
	case Errors:
		view = ErrorsView(err...)
	default:
		view = ErrorsView(err)
	}
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(view); err != nil {
		logrus.Error(err)
	}
	return true
}

type Errors []error

func (e Errors) Error() string {
	var msgs []string
	for _, err := range e {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, ", ")
}

type ErrorWithMetadata interface {
	ErrorMetadata() map[string]interface{}
}

func ErrorsView(errors ...error) errorsView {
	errorViews := []map[string]interface{}{}
	for _, err := range errors {
		errorViews = append(errorViews, errorView(err))
	}
	return errorsView{errorViews}
}

type errorsView struct {
	Errors []map[string]interface{} `json:"errors"`
}

func errorView(err error) map[string]interface{} {
	result := map[string]interface{}{}

	if m, hasMetadata := err.(ErrorWithMetadata); hasMetadata {
		for k, v := range m.ErrorMetadata() {
			result[k] = v
		}
	}

	result["message"] = err.Error()

	return result
}
