package main

import (
	"fmt"
	"html/template"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/dustinkirkland/golang-petname"
)

var templates *template.Template
var users map[string]*User

type User struct {
	Token         string
	AppName       string
	SessionID     string
	SessionExpiry time.Time
}

func main() {
	rand.Seed(time.Now().UnixNano())
	users = make(map[string]*User)

	var err error
	templates, err = template.ParseGlob("templates/*.html")
	if err != nil {
		logrus.Fatal(err)
	}

	http.HandleFunc("/users/signup", Signup)
	http.HandleFunc("/users/lookup", Lookup)

	logrus.Info("Listening on :3000")
	logrus.Fatal(http.ListenAndServe(":3000", nil))
}

func Signup(w http.ResponseWriter, r *http.Request) {
	data := make(map[string]interface{})
	if r.Method == "POST" {
		// TODO: Check this isn't being used to scrape our user database.
		email := r.FormValue("email")
		data["Email"] = email
		ensureUserExists(email)
		sendLoginEmail(email)
		data["Token"] = users[email].Token
	} else if token := r.FormValue("token"); token != "" {
		for _, user := range users {
			if user.Token == token {
				user.Token = ""
				user.SessionID = randomString()
				user.SessionExpiry = time.Now().UTC().Add(1440 * time.Hour)
				http.SetCookie(w, &http.Cookie{
					Name:  "weave_run_session_id",
					Value: user.SessionID,

					Expires: user.SessionExpiry,
				})
				// TODO: Set a session cookie here
				http.Redirect(w, r, fmt.Sprintf("/app/%s", user.AppName), http.StatusFound)
				return
			}
			data["TokenExpired"] = true
		}
	}
	executeTemplate(w, "signup.html", data)
}

// STUB
// TODO: Implement this.
func ensureUserExists(email string) {
	if _, ok := users[email]; !ok {
		users[email] = &User{
			AppName: fmt.Sprintf("%s-%d", petname.Generate(2, "-"), rand.Int31n(100)),
		}
	}
}

// STUB
// TODO: Implement this properly.
func sendLoginEmail(to string) {
	users[to].Token = randomString()
}

func randomString() string {
	return strconv.FormatUint(uint64(rand.Int63()), 36)
}

func executeTemplate(w http.ResponseWriter, templateName string, data interface{}) {
	t := templates.Lookup(templateName)
	if t == nil {
		logrus.Errorf("Template Not Found: %s", templateName)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if err := t.Execute(w, data); err != nil {
		logrus.Error(err)
	}
}

func Lookup(w http.ResponseWriter, r *http.Request) {
	sessionID := r.FormValue("session_id")
	if sessionID == "" {
		http.Error(w, "session_id param is required", http.StatusBadRequest)
		return
	}

	for _, user := range users {
		if user.SessionID == sessionID {
			if time.Now().After(user.SessionExpiry) {
				break
			}
			w.WriteHeader(http.StatusOK)
			return
		}
	}
	w.WriteHeader(http.StatusUnauthorized)
}
