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
		emailURI      string
		logLevel      string
		sessionSecret string
	)

	rand.Seed(time.Now().UnixNano())

	flag.StringVar(&emailURI, "email-uri", "smtp://smtp.weave.local:587", "uri of smtp server to send email through, of the format: smtp://username:password@hostname:port")
	flag.StringVar(&logLevel, "log-level", "info", "logging level (debug, info, warning, error)")
	flag.StringVar(&sessionSecret, "session-secret", "", "Secret used validate sessions")
	flag.Parse()

	setupLogging(logLevel)
	setupEmail(emailURI)
	setupStorage()
	setupTemplates()
	setupSessions(sessionSecret)
	logrus.Debug("Debug logging enabled")

	http.HandleFunc("/users/signup", Signup)
	http.HandleFunc("/users/lookup", Lookup)
	http.HandleFunc("/app/", App)

	logrus.Info("Listening on :3000")
	logrus.Info("Please visit: http://localhost:3000/users/signup")
	logrus.Fatal(http.ListenAndServe(":3000", nil))
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
			if token, err = storage.GenerateUserToken(user.ID); err == nil {
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
			} else if err := storage.ResetUserToken(user.ID); err != nil {
				logrus.Error(err)
			}
			http.Redirect(w, r, fmt.Sprintf("/app/%s", user.OrganizationName), http.StatusFound)
			return
		}
	}
	if err := executeTemplate(w, "signup.html", data); err != nil {
		internalServerError(w, err)
	}
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

func App(w http.ResponseWriter, r *http.Request) {
	user, err := sessions.Get(r)
	if err != nil {
		if err == ErrInvalidAuthenticationData {
			http.Redirect(w, r, "/users/signup", http.StatusFound)
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
