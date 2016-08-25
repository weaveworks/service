package api_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/jordan-wright/email"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/sessions"
)

func findLoginLink(t *testing.T, e *email.Email) (url, token string) {
	pattern := domain + `/#/login/[\w.%]+/([A-Za-z0-9%._=-]+)`
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(string(e.Text))
	require.Len(t, matches, 2, fmt.Sprintf("Could not find Login Link in text: %q", e.Text))
	require.NotEqual(t, "", matches[0])
	require.NotEqual(t, "", matches[1])
	require.Contains(t, string(e.HTML), matches[0], fmt.Sprintf("Could not find Login Link in html: %q", e.HTML))
	return matches[0], matches[1]
}

func newLoginRequest(t *testing.T, e *email.Email) *http.Request {
	loginLink, _ := findLoginLink(t, e)
	require.Contains(t, string(e.HTML), loginLink)

	u, err := url.Parse(loginLink)
	require.NoError(t, err)
	// convert email link /#/login/foo/bar to /api/users/login?email=foo&token=bar
	fragments := strings.Split(u.Fragment, "/")
	params := url.Values{}
	params.Set("email", fragments[2])
	params.Set("token", fragments[3])
	path := fmt.Sprintf("/api/users/login?%s", params.Encode())
	r, err := http.NewRequest("GET", path, nil)
	require.NoError(t, err)
	return r
}

// Check if a response has some named cookie
func hasCookie(w *httptest.ResponseRecorder, name string) bool {
	cookies := (&http.Response{Header: w.HeaderMap}).Cookies()
	for _, c := range cookies {
		if c.Name == name {
			return true
		}
	}
	return false
}

func Test_Signup(t *testing.T) {
	setup(t)
	defer cleanup(t)

	email := "joe@weave.works"

	// Signup as a new user, should send login email
	w := httptest.NewRecorder()
	r, _ := http.NewRequest("POST", "/api/users/signup", jsonBody{"email": email}.Reader(t))
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	body := map[string]interface{}{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, map[string]interface{}{"mailSent": true, "email": email}, body)
	require.Len(t, sentEmails, 1)
	user, err := db.FindUserByEmail(email)
	require.NoError(t, err)
	assert.Equal(t, []string{email}, sentEmails[0].To)
	loginLink, emailToken := findLoginLink(t, sentEmails[0])
	assert.Contains(t, string(sentEmails[0].HTML), loginLink)

	// Check they were immediately approved
	assert.False(t, user.ApprovedAt.IsZero(), "user should be approved")

	// Check the db one was hashed
	assert.NotEqual(t, "", user.Token, "user should have a token set")
	assert.NotEqual(t, user.Token, emailToken, "stored token should have been hashed")

	// Check the email one wasn't hashed (by looking for dollar-signs)
	assert.NotContains(t, emailToken, "$")
	assert.NotContains(t, emailToken, "%24")

	// Login with the link
	u, err := url.Parse(loginLink)
	assert.NoError(t, err)
	// convert email link /#/login/foo/bar to /api/users/login?email=foo&token=bar
	fragments := strings.Split(u.Fragment, "/")
	params := url.Values{}
	params.Set("email", fragments[2])
	params.Set("token", fragments[3])
	path := fmt.Sprintf("/api/users/login?%s", params.Encode())
	w = httptest.NewRecorder()
	r, _ = http.NewRequest("GET", path, nil)
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, hasCookie(w, sessions.CookieName))
	body = map[string]interface{}{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, map[string]interface{}{
		"firstLogin": true,
	}, body)

	user, err = db.FindUserByEmail(email)
	require.NoError(t, err)
	// Invalidates their login token
	assert.Equal(t, "", user.Token)
	// Sets their FirstLoginAt
	assert.False(t, user.FirstLoginAt.IsZero(), "Login should have set user's FirstLoginAt")
	firstLoginAt := user.FirstLoginAt
	// Doesn't create an organization.
	assert.Len(t, user.Organizations, 0)

	// Subsequent Logins do not change their FirstLoginAt or organization
	w = httptest.NewRecorder()
	r, _ = http.NewRequest("POST", "/api/users/signup", jsonBody{"email": email}.Reader(t))
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	require.Len(t, sentEmails, 2)
	assert.Equal(t, []string{email}, sentEmails[1].To)
	w = httptest.NewRecorder()
	app.ServeHTTP(w, newLoginRequest(t, sentEmails[1]))
	assert.Equal(t, http.StatusOK, w.Code)
	body = map[string]interface{}{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, map[string]interface{}{}, body)

	user, err = db.FindUserByEmail(email)
	require.NoError(t, err)
	assert.False(t, user.FirstLoginAt.IsZero(), "Login should have set user's FirstLoginAt")
	assert.Equal(t, firstLoginAt, user.FirstLoginAt, "Second login should not have changed user's FirstLoginAt")
	assert.Len(t, user.Organizations, 0)
}

func Test_Signup_WithInvalidJSON(t *testing.T) {
	setup(t)
	defer cleanup(t)

	email := ""
	w := httptest.NewRecorder()
	r, _ := http.NewRequest("POST", "/api/users/signup", strings.NewReader("this isn't json"))

	_, err := db.FindUserByEmail(email)
	assert.EqualError(t, err, users.ErrNotFound.Error())
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "invalid character 'h' in literal true (expecting 'r')")
	_, err = db.FindUserByEmail(email)
	assert.EqualError(t, err, users.ErrNotFound.Error())
}

func Test_Signup_WithBlankEmail(t *testing.T) {
	setup(t)
	defer cleanup(t)

	email := ""
	w := httptest.NewRecorder()
	r, _ := http.NewRequest("POST", "/api/users/signup", jsonBody{}.Reader(t))

	_, err := db.FindUserByEmail(email)
	assert.EqualError(t, err, users.ErrNotFound.Error())
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Email cannot be blank")
	_, err = db.FindUserByEmail(email)
	assert.EqualError(t, err, users.ErrNotFound.Error())
}

func Test_Signup_ViaOAuth(t *testing.T) {
	setup(t)
	defer cleanup(t)

	email := "joe@example.com"
	logins.Register("mock", MockLoginProvider{
		"joe": {ID: "joe", Email: email},
	})

	// Signup as a new user via oauth, should *not* send welcome email
	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "/api/users/logins/mock/attach?code=joe&state=state", nil)
	_, err := db.FindUserByEmail(email)
	assert.EqualError(t, err, users.ErrNotFound.Error())
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, hasCookie(w, sessions.CookieName))
	body := map[string]interface{}{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, map[string]interface{}{
		"firstLogin":  true,
		"userCreated": true,
	}, body)
	assert.Len(t, sentEmails, 0)

	user, err := db.FindUserByEmail(email)
	require.NoError(t, err)

	assert.False(t, user.ApprovedAt.IsZero(), "user should be approved")
	assert.Equal(t, "", user.Token, "user should not have a token set")
	assert.False(t, user.FirstLoginAt.IsZero(), "Login should have set user's FirstLoginAt")

	// User should have login set
	if assert.Len(t, user.Logins, 1) {
		assert.Equal(t, user.ID, user.Logins[0].UserID)
		assert.Equal(t, "mock", user.Logins[0].Provider)
		assert.Equal(t, "joe", user.Logins[0].ProviderID)
	}
}

func Test_Signup_ViaOAuth_MatchesByEmail(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user := getApprovedUser(t)
	logins.Register("mock", MockLoginProvider{
		"joe": {ID: "joe", Email: user.Email},
	})
	// User should not have any logins yet.
	assert.Len(t, user.Logins, 0)

	// Signup as an existing user via oauth, should match with existing user
	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "/api/users/logins/mock/attach?code=joe&state=state", nil)
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, hasCookie(w, sessions.CookieName))
	assert.Len(t, sentEmails, 0)

	found, err := db.FindUserByEmail(user.Email)
	require.NoError(t, err)
	body := map[string]interface{}{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, map[string]interface{}{
		"firstLogin": true,
	}, body)

	assert.Equal(t, user.ID, found.ID, "user id should match the existing")
	assert.False(t, found.ApprovedAt.IsZero(), "user should be approved")
	assert.Equal(t, user.Token, found.Token, "user should still have same token set")
	assert.False(t, found.FirstLoginAt.IsZero(), "Login should have set user's FirstLoginAt")

	// User should have a login set
	if assert.Len(t, found.Logins, 1) {
		assert.Equal(t, user.ID, found.Logins[0].UserID)
		assert.Equal(t, "mock", found.Logins[0].Provider)
		assert.Equal(t, "joe", found.Logins[0].ProviderID)
	}
}

func Test_Signup_ViaOAuth_EmailChanged(t *testing.T) {
	// When a user has changed their remote email, but the remote user ID is the same.
	setup(t)
	defer cleanup(t)
	user := getApprovedUser(t)
	provider := MockLoginProvider{
		"joe": {ID: "joe", Email: user.Email},
	}
	logins.Register("mock", provider)

	require.NoError(t, db.AddLoginToUser(user.ID, "mock", "joe", nil))

	// Change the remote email
	newEmail := "fran@example.com"
	provider["joe"] = MockRemoteUser{ID: "joe", Email: newEmail}

	// Login as an existing user with remote email changed
	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "/api/users/logins/mock/attach?code=joe&state=state", nil)
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, hasCookie(w, sessions.CookieName))
	assert.Len(t, sentEmails, 0)

	_, err := db.FindUserByEmail(newEmail)
	assert.EqualError(t, err, users.ErrNotFound.Error())

	user, err = db.FindUserByID(user.ID)
	require.NoError(t, err)
	// User should have a login set
	if assert.Len(t, user.Logins, 1) {
		assert.Equal(t, user.ID, user.Logins[0].UserID)
		assert.Equal(t, "mock", user.Logins[0].Provider)
		assert.Equal(t, "joe", user.Logins[0].ProviderID)
	}
}
