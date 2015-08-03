package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
)

var (
	sessions            SessionStore
	storage             Storage
	domain              = "localhost"
	passwordHashingCost = 14
)

func main() {
	var (
		addr          = flag.String("addr", "0.0.0.0:80", "interface and port to listen on")
		databaseURI   = flag.String("database-uri", "postgres://postgres@db.weave.local/weave_development?sslmode=disable", "URI where the database can be found (for dev you can use memory://)")
		emailURI      = flag.String("email-uri", "smtp://smtp.weave.local:587", "uri of smtp server to send email through, of the format: smtp://username:password@hostname:port")
		logLevel      = flag.String("log-level", "info", "logging level (debug, info, warning, error)")
		sessionSecret = flag.String("session-secret", "", "Secret used validate sessions")
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

	logrus.Infof("Listening on %s", *addr)
	logrus.Fatal(http.ListenAndServe(*addr, handler()))
}

func handler() http.Handler {
	return loggingMiddleware(routes())
}

func routes() http.Handler {
	r := mux.NewRouter()
	r.HandleFunc("/api/users/signup", Signup).Methods("POST")
	r.HandleFunc("/api/users/login", Login).Methods("GET")
	r.HandleFunc("/api/users/private/lookup", Lookup).Methods("GET")
	r.HandleFunc("/api/users/private/users", ListUsers).Methods("GET")
	r.HandleFunc("/api/users/private/users/{userID}/approve", ApproveUser).Methods("POST")
	r.HandleFunc("/api/users/org/{orgID}", Org).Methods("GET")
	return r
}

type signupView struct {
	MailSent bool   `json:"mailSent"`
	Email    string `json:"email,omitempty"`
}

func Signup(w http.ResponseWriter, r *http.Request) {
	view := signupView{
		MailSent: false,
		Email:    r.FormValue("email"),
	}
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
	}
	if user.ApprovedAt.IsZero() {
		err = SendWelcomeEmail(user)
	} else {
		err = SendLoginEmail(user, token)
	}
	if err != nil {
		logrus.Error(err)
		renderError(w, http.StatusInternalServerError, fmt.Errorf("Error sending login email"))
	} else {
		view.MailSent = true
	}

	renderJSON(w, http.StatusOK, view)
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
	if email == "" || token == "" {
		http.Redirect(w, r, "/signup", http.StatusFound)
		return
	}

	tokenExpired := func(errs ...error) {
		for _, err := range errs {
			if err != nil {
				logrus.Error(err)
			}
		}
		http.Redirect(w, r, "/signup?token_expired=true", http.StatusFound)
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
	http.Redirect(w, r, fmt.Sprintf("/org/%s", user.OrganizationName), http.StatusFound)
}

type lookupView struct {
	OrganizationID string `json:"organizationID"`
}

func Lookup(w http.ResponseWriter, r *http.Request) {
	sessionID := r.FormValue("session_id")
	if sessionID == "" {
		http.Error(w, "session_id param is required", http.StatusBadRequest)
		return
	}

	user, err := sessions.Decode(sessionID)
	switch {
	case err == nil:
		renderJSON(w, http.StatusOK, lookupView{OrganizationID: user.OrganizationID})
	case err == ErrInvalidAuthenticationData:
		w.WriteHeader(http.StatusUnauthorized)
	default:
		internalServerError(w, err)
	}
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
	err := storage.ApproveUser(userID)
	switch {
	case err == ErrNotFound:
		renderError(w, http.StatusNotFound, fmt.Errorf("Not Found"))
	case err != nil:
		internalServerError(w, err)
	default:
		http.Redirect(w, r, "/api/users/private/users", http.StatusFound)
	}
}

type orgView struct {
	User string `json:"user"`
	Name string `json:"name"`
}

func Org(w http.ResponseWriter, r *http.Request) {
	user, err := sessions.Get(r)
	if err != nil {
		if err == ErrInvalidAuthenticationData {
			http.Redirect(w, r, "/api/users/signup", http.StatusFound)
		} else {
			internalServerError(w, err)
		}
		return
	}
	renderJSON(w, http.StatusOK, orgView{
		User: user.Email,
		Name: user.OrganizationName,
	})
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

func renderError(w http.ResponseWriter, status int, err error) {
	w.WriteHeader(status)
	data := map[string]interface{}{
		"errors": []map[string]interface{}{
			{
				"message": err.Error(),
			},
		},
	}
	if err := json.NewEncoder(w).Encode(data); err != nil {
		logrus.Error(err)
	}
}
