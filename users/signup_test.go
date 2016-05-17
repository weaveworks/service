package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/jordan-wright/email"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weaveworks/service/users/login"
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

func jsonBody(t *testing.T, data interface{}) io.Reader {
	b, err := json.Marshal(data)
	require.NoError(t, err)
	return bytes.NewReader(b)
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

	// Signup as a new user, should send welcome email
	w := httptest.NewRecorder()
	r, _ := http.NewRequest("POST", "/api/users/signup", jsonBody(t, map[string]interface{}{"email": "joe@weave.works"}))
	_, err := storage.FindUserByEmail(email)
	assert.EqualError(t, err, errNotFound.Error())
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	body := map[string]interface{}{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, map[string]interface{}{"mailSent": true, "email": email}, body)

	user, err := storage.FindUserByEmail(email)
	require.NoError(t, err)
	assert.True(t, user.ApprovedAt.IsZero(), "user should not be approved")
	require.Len(t, user.Organizations, 0)
	if assert.Len(t, sentEmails, 1) {
		assert.Equal(t, fromAddress, sentEmails[0].From)
		assert.Equal(t, []string{email}, sentEmails[0].To)
		assert.Contains(t, string(sentEmails[0].Text), "Thanks for your interest")
		assert.Contains(t, string(sentEmails[0].HTML), "Thanks for your interest")
	}

	// Manually approve
	user, err = storage.ApproveUser(user.ID)
	require.NoError(t, err)
	require.Len(t, user.Organizations, 0)
	assert.False(t, user.ApprovedAt.IsZero(), "user should be approved")

	// Do it again: check it preserves their data, and sends a login email
	w = httptest.NewRecorder()
	r, _ = http.NewRequest("POST", "/api/users/signup", jsonBody(t, map[string]interface{}{"email": "joe@weave.works"}))
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	body = map[string]interface{}{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, map[string]interface{}{"mailSent": true, "email": email}, body)
	require.Len(t, sentEmails, 2)
	user, err = storage.FindUserByEmail(email)
	require.NoError(t, err)
	assert.Equal(t, []string{email}, sentEmails[1].To)
	loginLink, emailToken := findLoginLink(t, sentEmails[1])
	assert.Contains(t, string(sentEmails[1].HTML), loginLink)

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
	assert.True(t, hasCookie(w, cookieName))
	body = map[string]interface{}{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, email, body["email"])
	assert.Equal(t, "/signup/success", body["redirectTo"])
	assert.Len(t, body["organizations"], 1)

	user, err = storage.FindUserByEmail(email)
	require.NoError(t, err)
	// Invalidates their login token
	assert.Equal(t, "", user.Token)
	// Sets their FirstLoginAt
	assert.False(t, user.FirstLoginAt.IsZero(), "Login should have set user's FirstLoginAt")
	firstLoginAt := user.FirstLoginAt
	// Creates their first organization
	require.Len(t, user.Organizations, 1)
	orgName := user.Organizations[0].Name

	// Subsequent Logins do not change their FirstLoginAt or organization
	w = httptest.NewRecorder()
	r, _ = http.NewRequest("POST", "/api/users/signup", jsonBody(t, map[string]interface{}{"email": "joe@weave.works"}))
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	require.Len(t, sentEmails, 3)
	assert.Equal(t, []string{email}, sentEmails[2].To)
	w = httptest.NewRecorder()
	app.ServeHTTP(w, newLoginRequest(t, sentEmails[2]))
	assert.Equal(t, http.StatusOK, w.Code)
	body = map[string]interface{}{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, email, body["email"])
	assert.Equal(t, "/login/success", body["redirectTo"])
	assert.Len(t, body["organizations"], 1)

	user, err = storage.FindUserByEmail(email)
	require.NoError(t, err)
	assert.False(t, user.FirstLoginAt.IsZero(), "Login should have set user's FirstLoginAt")
	assert.Equal(t, firstLoginAt, user.FirstLoginAt, "Second login should not have changed user's FirstLoginAt")
	assert.Equal(t, orgName, user.Organizations[0].Name)
}

func Test_Signup_WithInvalidJSON(t *testing.T) {
	setup(t)
	defer cleanup(t)

	email := ""
	w := httptest.NewRecorder()
	r, _ := http.NewRequest("POST", "/api/users/signup", strings.NewReader("this isn't json"))

	_, err := storage.FindUserByEmail(email)
	assert.EqualError(t, err, errNotFound.Error())
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "invalid character 'h' in literal true (expecting 'r')")
	_, err = storage.FindUserByEmail(email)
	assert.EqualError(t, err, errNotFound.Error())
}

func Test_Signup_WithBlankEmail(t *testing.T) {
	setup(t)
	defer cleanup(t)

	email := ""
	w := httptest.NewRecorder()
	r, _ := http.NewRequest("POST", "/api/users/signup", jsonBody(t, map[string]interface{}{}))

	_, err := storage.FindUserByEmail(email)
	assert.EqualError(t, err, errNotFound.Error())
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Email cannot be blank")
	_, err = storage.FindUserByEmail(email)
	assert.EqualError(t, err, errNotFound.Error())
}

func Test_Signup_ViaOAuth(t *testing.T) {
	setup(t)
	defer cleanup(t)

	email := "joe@example.com"
	login.Register("mock", MockLoginProvider{
		"joe": {ID: "joe", Email: email},
	})

	// Signup as a new user via oauth, should *not* send welcome email
	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "/api/users/login/mock?code=joe&state=state", nil)
	_, err := storage.FindUserByEmail(email)
	assert.EqualError(t, err, errNotFound.Error())
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, hasCookie(w, cookieName))
	body := map[string]interface{}{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, email, body["email"])
	assert.Equal(t, "/signup/success", body["redirectTo"])
	assert.Len(t, body["organizations"], 1)
	assert.Len(t, sentEmails, 0)

	user, err := storage.FindUserByEmail(email)
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
	email := "joe@example.com"
	login.Register("mock", MockLoginProvider{
		"joe": {ID: "joe", Email: email},
	})

	user, err := storage.CreateUser("", "joe@example.com")
	require.NoError(t, err)
	// User should not have any logins yet.
	assert.Len(t, user.Logins, 0)

	// Signup as an existing user via oauth, should match with existing user
	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "/api/users/login/mock?code=joe&state=state", nil)
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, hasCookie(w, cookieName))
	assert.Len(t, sentEmails, 0)

	found, err := storage.FindUserByEmail(email)
	require.NoError(t, err)
	body := map[string]interface{}{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, email, body["email"])
	assert.Equal(t, "/signup/success", body["redirectTo"])
	assert.Len(t, body["organizations"], 1)

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
	email := "joe@example.com"
	provider := MockLoginProvider{
		"joe": {ID: "joe", Email: email},
	}
	login.Register("mock", provider)

	user, err := storage.CreateUser("", "joe@example.com")
	require.NoError(t, err)
	require.NoError(t, storage.AddLoginToUser(user.ID, "mock", "joe", nil))

	// Change the remote email
	newEmail := "fran@example.com"
	provider["joe"] = MockRemoteUser{ID: "joe", Email: newEmail}

	// Login as an existing user with remote email changed
	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "/api/users/login/mock?code=joe&state=state", nil)
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, hasCookie(w, cookieName))
	assert.Len(t, sentEmails, 0)

	_, err = storage.FindUserByEmail(newEmail)
	assert.EqualError(t, err, errNotFound.Error())

	user, err = storage.FindUserByID(user.ID)
	require.NoError(t, err)
	// User should have a login set
	if assert.Len(t, user.Logins, 1) {
		assert.Equal(t, user.ID, user.Logins[0].UserID)
		assert.Equal(t, "mock", user.Logins[0].Provider)
		assert.Equal(t, "joe", user.Logins[0].ProviderID)
	}
}
