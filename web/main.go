package main

import (
	"flag"
	"fmt"
	htmlTemplate "html/template"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/Sirupsen/logrus"
	"github.com/dustinkirkland/golang-petname"
)

var (
	users               map[string]*User
	sessions            SessionStore
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
	users = make(map[string]*User)

	flag.StringVar(&emailURI, "email-uri", "smtp://smtp.weave.local:587", "uri of smtp server to send email through, of the format: smtp://username:password@hostname:port")
	flag.StringVar(&logLevel, "log-level", "info", "logging level (debug, info, warning, error)")
	flag.StringVar(&sessionSecret, "session-secret", "", "Secret used validate sessions")
	flag.Parse()

	setupLogging(logLevel)
	setupEmail(emailURI)
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

type User struct {
	ID               string
	Email            string
	Token            string
	ApprovedAt       time.Time
	OrganizationID   string
	OrganizationName string
}

// TODO: Use something more secure than randomString
func (u *User) GenerateToken() (string, error) {
	raw := randomString()
	hashed, err := bcrypt.GenerateFromPassword([]byte(raw), passwordHashingCost)
	if err != nil {
		return "", err
	}
	u.Token = string(hashed)
	return raw, nil
}

func (u *User) CompareToken(other string) bool {
	return bcrypt.CompareHashAndPassword([]byte(u.Token), []byte(other)) == nil
}

func (u *User) LoginURL(rawToken string) string {
	params := url.Values{}
	params.Set("email", u.Email)
	params.Set("token", rawToken)
	return fmt.Sprintf("http://%s/users/signup?%s", domain, params.Encode())
}

func (u *User) LoginLink(rawToken string) htmlTemplate.HTML {
	url := u.LoginURL(rawToken)
	return htmlTemplate.HTML(
		fmt.Sprintf(
			"<a href=\"%s\">%s</a>",
			url,
			htmlTemplate.HTMLEscapeString(url),
		),
	)
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
		mailer := SendLoginEmail
		user, ok := users[email]
		if !ok || user.ApprovedAt.IsZero() {
			mailer = SendWelcomeEmail
			user = createUser(email)
		}
		data["User"] = user
		if err := mailer(user); err != nil {
			logrus.Error(err)
			data["LoginEmailSent"] = false
			data["ErrorSendingLoginEmail"] = true
		}
	} else if token := r.FormValue("token"); token != "" {
		for _, user := range users {
			if user.Email == email && user.CompareToken(token) {
				user.Token = ""
				if err := sessions.Set(w, user.ID); err != nil {
					logrus.Error(err)
				}
				http.Redirect(w, r, fmt.Sprintf("/app/%s", user.OrganizationName), http.StatusFound)
				return
			}
			data["TokenExpired"] = true
		}
	}
	if err := executeTemplate(w, "signup.html", data); err != nil {
		internalServerError(w, err)
	}
}

// TODO: Implement this.
func createUser(email string) *User {
	users[email] = &User{
		Email:            email,
		OrganizationName: fmt.Sprintf("%s-%d", petname.Generate(2, "-"), rand.Int31n(100)),
	}
	return users[email]
}

// TODO: Replace this for security where needed.
func randomString() string {
	return strconv.FormatUint(uint64(rand.Int63()), 36)
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
