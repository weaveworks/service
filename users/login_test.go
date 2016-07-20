package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/weaveworks/service/users/login"
)

// MockLoginProvider is used in testing. It just authenticates anyone.
type MockLoginProvider map[string]MockRemoteUser

type MockRemoteUser struct {
	ID, Email string
}

// Flags sets the flags this provider requires on the command-line
func (a MockLoginProvider) Flags(*flag.FlagSet) {}

// Name is the human-readable name of the provider
func (a MockLoginProvider) Name() string { return "mock" }

// Link we should render for this provider flow to begin.
func (a MockLoginProvider) Link(r *http.Request) (login.Link, bool) {
	return login.Link{}, false
}

// Login converts a user to a db ID and email
func (a MockLoginProvider) Login(r *http.Request) (id, email string, session json.RawMessage, err error) {
	code := r.FormValue("code")
	u, ok := a[code]
	if !ok {
		return "", "", nil, errInvalidAuthenticationData
	}
	session, err = json.Marshal(u.ID)
	if err != nil {
		return "", "", nil, err
	}
	return u.ID, u.Email, session, nil
}

// Username fetches a user's username on the remote service, for displaying *which* account this is linked with.
func (a MockLoginProvider) Username(session json.RawMessage) (string, error) {
	var id string
	if err := json.Unmarshal(session, &id); err != nil {
		return "", err
	}

	for _, u := range a {
		if u.ID == id {
			return u.Email, nil
		}
	}
	return "", errNotFound
}

// Logout handles a user logout request with this provider. It should return
// detach revoke the user session, requiring the user to re-authenticate next
// time.
func (a MockLoginProvider) Logout(session json.RawMessage) error {
	var id string
	if err := json.Unmarshal(session, &id); err != nil {
		return fmt.Errorf("Error logging out: %s, Session: %q", err.Error(), string(session))
	}

	var ks []string
	for k, u := range a {
		if u.ID == id {
			ks = append(ks, k)
		}
	}
	for _, k := range ks {
		delete(a, k)
	}
	return nil
}

func Test_Login_NoParams(t *testing.T) {
	setup(t)
	defer cleanup(t)

	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "/api/users/login", nil)
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), `{"errors":[{"message":"Email cannot be blank"}]}`)
}

func Test_Login_Unauthorized(t *testing.T) {
	setup(t)
	defer cleanup(t)

	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "/api/users/login?email=joe@weave.works&token=foo", nil)
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), `{"errors":[{"message":"Invalid authentication data"}]}`)
}
