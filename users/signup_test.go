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

func Test_Signup(t *testing.T) {
	setup(t)
	defer cleanup(t)

	email := "joe@weave.works"

	// Signup as a new user, should send login email
	w := httptest.NewRecorder()
	r, _ := http.NewRequest("POST", "/api/users/signup", jsonBody(t, map[string]interface{}{"email": email}))
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	body := map[string]interface{}{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, map[string]interface{}{"mailSent": true, "email": email}, body)
	require.Len(t, sentEmails, 1)
	user, err := storage.FindUserByEmail(email)
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

	assert.NotEqual(t, "", user.Organization.ProbeToken, "user should have a probe token")

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
	assert.True(t, strings.HasPrefix(w.HeaderMap.Get("Set-Cookie"), cookieName+"="))
	body = map[string]interface{}{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, map[string]interface{}{"email": email, "organizationName": user.Organization.Name}, body)

	user, err = storage.FindUserByEmail(email)
	require.NoError(t, err)
	// Invalidates their login token
	assert.Equal(t, "", user.Token)
	// Sets their FirstLoginAt
	assert.False(t, user.FirstLoginAt.IsZero(), "Login should have set user's FirstLoginAt")
	firstLoginAt := user.FirstLoginAt

	// Subsequent Logins do not change their FirstLoginAt
	w = httptest.NewRecorder()
	r, _ = http.NewRequest("POST", "/api/users/signup", jsonBody(t, map[string]interface{}{"email": "joe@weave.works"}))
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	require.Len(t, sentEmails, 2)
	assert.Equal(t, []string{email}, sentEmails[1].To)
	w = httptest.NewRecorder()
	app.ServeHTTP(w, newLoginRequest(t, sentEmails[1]))
	assert.Equal(t, http.StatusOK, w.Code)

	user, err = storage.FindUserByEmail(email)
	require.NoError(t, err)
	assert.False(t, user.FirstLoginAt.IsZero(), "Login should have set user's FirstLoginAt")
	assert.Equal(t, firstLoginAt, user.FirstLoginAt, "Second login should not have changed user's FirstLoginAt")
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
