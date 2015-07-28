package main

import (
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/Sirupsen/logrus"
)

var (
	sessions            SessionStore
	storage             Storage
	domain              = "localhost"
	passwordHashingCost = 14
)

func main() {
	var (
		databaseURI   string
		emailURI      string
		logLevel      string
		sessionSecret string
	)

	rand.Seed(time.Now().UnixNano())

	flag.StringVar(&databaseURI, "database-uri", "postgres://postgres@db.weave.local/weave_development?sslmode=disable", "URI where the database can be found")
	flag.StringVar(&emailURI, "email-uri", "smtp://smtp.weave.local:587", "uri of smtp server to send email through, of the format: smtp://username:password@hostname:port")
	flag.StringVar(&logLevel, "log-level", "info", "logging level (debug, info, warning, error)")
	flag.StringVar(&sessionSecret, "session-secret", "", "Secret used validate sessions")
	flag.Parse()

	setupLogging(logLevel)
	setupEmail(emailURI)
	setupStorage(databaseURI)
	defer storage.Close()
	setupTemplates()
	setupSessions(sessionSecret)
	logrus.Debug("Debug logging enabled")

	http.HandleFunc("/api/users/signup", Signup)
	http.HandleFunc("/api/users/private/lookup", Lookup)
	http.HandleFunc("/api/users/org/", Org)

	logrus.Info("Listening on :80")
	logrus.Fatal(http.ListenAndServe(":80", nil))
}

func Signup(w http.ResponseWriter, r *http.Request) {
	data := make(map[string]interface{})
	email := r.FormValue("email")
	if r.Method == "POST" {
		if email == "" {
			data["EmailBlank"] = true
			w.WriteHeader(http.StatusBadRequest)
			if err := executeTemplate(w, "signup.html", data); err != nil {
				internalServerError(w, err)
			}
			return
		}
		data["Email"] = email
		data["LoginEmailSent"] = true

		user, err := storage.FindUserByEmail(email)
		if err == ErrNotFound {
			user, err = storage.CreateUser(email)
		}
		if err != nil {
			internalServerError(w, err)
			return
		}
		if user.ApprovedAt.IsZero() {
			err = SendWelcomeEmail(user)
		} else {
			var token string
			if token, err = generateUserToken(storage, user); err == nil {
				err = SendLoginEmail(user, token)
			}
		}
		if err != nil {
			logrus.Error(err)
			data["LoginEmailSent"] = false
			data["ErrorSendingLoginEmail"] = true
		}
	} else if token := r.FormValue("token"); token != "" {
		data["TokenExpired"] = true
		user, err := storage.FindUserByEmail(email)
		if err != ErrNotFound && err != nil {
			logrus.Error(err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		if err == nil && user.CompareToken(token) {
			data["TokenExpired"] = false
			if err := sessions.Set(w, user.ID); err != nil {
				logrus.Error(err)
			} else if err := storage.SetUserToken(user.ID, ""); err != nil {
				logrus.Error(err)
			}
			http.Redirect(w, r, fmt.Sprintf("/api/users/org/%s", user.OrganizationName), http.StatusFound)
			return
		}
	}
	if err := executeTemplate(w, "signup.html", data); err != nil {
		internalServerError(w, err)
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

func Lookup(w http.ResponseWriter, r *http.Request) {
	sessionID := r.FormValue("session_id")
	if sessionID == "" {
		http.Error(w, "session_id param is required", http.StatusBadRequest)
		return
	}

	_, err := sessions.Decode(sessionID)
	switch {
	case err == nil:
		w.WriteHeader(http.StatusOK)
	case err == ErrInvalidAuthenticationData:
		w.WriteHeader(http.StatusUnauthorized)
	default:
		internalServerError(w, err)
	}
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
	if err := executeTemplate(w, "app.html", user); err != nil {
		logrus.Error(err)
	}
}

func internalServerError(w http.ResponseWriter, err error) {
	logrus.Error(err)
	http.Error(w, "Internal Server Error", http.StatusInternalServerError)
}
