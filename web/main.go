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

const (
	cookieName = "_weave_run_session"
)

var (
	users               map[string]*User
	domain              = "localhost"
	passwordHashingCost = 14
)

func main() {
	var (
		emailURI string
		logLevel string
	)

	rand.Seed(time.Now().UnixNano())
	users = make(map[string]*User)

	flag.StringVar(&emailURI, "email-uri", "smtp://smtp.weave.local:587", "uri of smtp server to send email through, of the format: smtp://username:password@hostname:port")
	flag.StringVar(&logLevel, "log-level", "info", "logging level (debug, info, warning, error)")
	flag.Parse()

	setupLogging(logLevel)
	setupEmail(emailURI)
	setupTemplates()
	logrus.Debug("Debug logging enabled")

	http.HandleFunc("/users/signup", Signup)
	http.HandleFunc("/users/lookup", Lookup)
	http.HandleFunc("/app/", App)

	logrus.Info("Listening on :3000")
	logrus.Info("Please visit: http://localhost:3000/users/signup")
	logrus.Fatal(http.ListenAndServe(":3000", nil))
}

type User struct {
	Email         string
	Token         string
	AppName       string
	SessionID     string
	SessionExpiry time.Time
	ApprovedAt    time.Time
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
				logrus.Error(err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
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
				user.SessionID = randomString()
				user.SessionExpiry = time.Now().UTC().Add(1440 * time.Hour)
				http.SetCookie(w, &http.Cookie{
					Name:    cookieName,
					Value:   user.SessionID,
					Path:    "/",
					Expires: user.SessionExpiry,
				})
				http.Redirect(w, r, fmt.Sprintf("/app/%s", user.AppName), http.StatusFound)
				return
			}
			data["TokenExpired"] = true
		}
	}
	if err := executeTemplate(w, "signup.html", data); err != nil {
		logrus.Error(err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// TODO: Implement this.
func createUser(email string) *User {
	users[email] = &User{
		Email:   email,
		AppName: fmt.Sprintf("%s-%d", petname.Generate(2, "-"), rand.Int31n(100)),
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

	if authenticate(sessionID) != nil {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusUnauthorized)
	}
}

func authenticate(sessionID string) *User {
	for _, user := range users {
		if user.SessionID == sessionID {
			if time.Now().After(user.SessionExpiry) {
				break
			}
			return user
		}
	}
	return nil
}

func App(w http.ResponseWriter, r *http.Request) {
	var data *User
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		http.Redirect(w, r, "/users/signup", http.StatusFound)
		return
	}
	user := authenticate(cookie.Value)
	if user == nil {
		http.Redirect(w, r, "/users/signup", http.StatusFound)
		return
	}
	data = user
	if err := executeTemplate(w, "app.html", data); err != nil {
		logrus.Error(err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
