package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/login"
)

// MockLoginProvider is used in testing. It just authenticates anyone.
type MockLoginProvider struct {
	Users                map[string]login.Claims
	LoggedInPasswordless []string
	Invited              []string
}

func (a *MockLoginProvider) SetUsers(users map[string]login.Claims) {
	a.Users = users
	a.LoggedInPasswordless = []string{}
	a.Invited = []string{}
}

func (a *MockLoginProvider) LoginURL(r *http.Request, connection string) string {
	return "/mock-login/"
}

func (a *MockLoginProvider) LogoutURL(r *http.Request) string {
	return "no escape"
}

// Login converts a user to a db ID and email
func (a *MockLoginProvider) Login(r *http.Request) (*login.Claims, json.RawMessage, map[string]string, error) {
	code := r.FormValue("code")
	u, ok := a.Users[code]
	if !ok {
		return nil, nil, nil, users.ErrInvalidAuthenticationData
	}
	session, err := json.Marshal(u.ID)
	if err != nil {
		return nil, nil, nil, err
	}
	return &u, session, make(map[string]string), nil
}

func (a *MockLoginProvider) GetAccessToken(user string) (*string, error) {
	token := "token"
	return &token, nil
}

func (a *MockLoginProvider) InviteUser(email string, inviter string, teamName string) error {
	a.Invited = append(a.Invited, email)
	return nil
}

func (a *MockLoginProvider) UpdateClaims(ctx context.Context, claims login.Claims, session json.RawMessage) error {
	return nil
}

func (a *MockLoginProvider) PasswordlessLogin(r *http.Request, email string) error {
	a.LoggedInPasswordless = append(a.LoggedInPasswordless, email)
	a.Users[email] = login.Claims{ID: "email|" + email, Email: email}
	return nil
}

func Test_Verify(t *testing.T) {
	setup(t)
	defer cleanup(t)

	type TestCase struct {
		requestURL        string
		redirectURL       string
		sessionParameters *[4]string
	}

	for _, testCase := range []TestCase{
		{
			"/api/users/verify",
			"/login",
			nil,
		},
		{
			"/api/users/verify?next=/instances",
			"/login?next=%2Finstances",
			nil,
		},
		{
			"/api/users/verify?next=/instances&unknown_parameter=ignored",
			"/login?next=%2Finstances",
			nil,
		},
		{
			"/api/users/verify",
			"/",
			&[4]string{"email", "test@test.test", "1", ""},
		},
		{
			"/api/users/verify?next=/instances",
			"/instances",
			&[4]string{"email", "test@test.test", "1", ""},
		},
		{
			"/api/users/verify?next=/instances&connection=email",
			"/instances",
			&[4]string{"email", "test@test.test", "1", ""},
		},
		{
			"/api/users/verify?next=/instances&connection=google",
			"/mock-login/",
			&[4]string{"email", "test@test.test", "1", ""},
		},
	} {
		w := httptest.NewRecorder()
		r, _ := http.NewRequest("GET", testCase.requestURL, nil)
		if testCase.sessionParameters != nil {
			s, _ := sessionStore.Cookie(
				testCase.sessionParameters[0],
				testCase.sessionParameters[1],
				testCase.sessionParameters[2],
				testCase.sessionParameters[3],
			)
			r.AddCookie(s)
		}
		app.ServeHTTP(w, r)
		assert.Equal(t, http.StatusSeeOther, w.Code)
		loc, _ := w.Result().Location()
		assert.Equal(t, loc.String(), testCase.redirectURL)
	}
}
