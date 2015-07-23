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

	"github.com/Sirupsen/logrus"
	"github.com/dustinkirkland/golang-petname"
)

const (
	cookieName = "_weave_run_session"
)

var (
	users  map[string]*User
	domain = "localhost"
)

func main() {
	var (
		err      error
		emailURI string
	)

	rand.Seed(time.Now().UnixNano())
	users = make(map[string]*User)

	flag.StringVar(&emailURI, "email-uri", "smtp://smtp.weave.local:587", "uri of smtp server to send email through, of the format: smtp://username:password@hostname:port")
	flag.Parse()
	sendEmail, err = smtpEmailSender(emailURI)
	if err != nil {
		logrus.Fatal(err)
	}

	if err := loadTemplates(); err != nil {
		logrus.Fatal(err)
	}

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

func (u *User) LoginLink() htmlTemplate.HTML {
	params := url.Values{}
	params.Set("email", u.Email)
	params.Set("token", u.Token)
	link := fmt.Sprintf("http://%s/users/signup?%s", domain, params.Encode())
	return htmlTemplate.HTML(
		fmt.Sprintf(
			"<a href=\"%s\">%s</a>",
			link,
			htmlTemplate.HTMLEscapeString(link),
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
			if user.Email == email && user.Token == token {
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
	if cookie, err := r.Cookie(cookieName); err != nil {
		http.Redirect(w, r, "/users/signup", http.StatusFound)
		return
	}
	if user := authenticate(cookie.Value); user == nil {
		http.Redirect(w, r, "/users/signup", http.StatusFound)
		return
	}
	data = user
	if err := executeTemplate(w, "app.html", data); err != nil {
		logrus.Error(err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
