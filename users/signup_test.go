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
	pattern := `http://` + domain + `/login\?email=[\w.%]+&token=([A-Za-z0-9%._=-]+)`
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

func Test_Signup(t *testing.T) {
	setup(t)
	defer cleanup(t)

	email := "joe@weave.works"

	// Signup as a new user, should send welcome email
	w := httptest.NewRecorder()
	r, _ := http.NewRequest("POST", "/api/users/signup", jsonBody(t, map[string]interface{}{"email": "joe@weave.works"}))
	_, err := storage.FindUserByEmail(email)
	assert.EqualError(t, err, ErrNotFound.Error())
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	body := map[string]interface{}{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, map[string]interface{}{"mailSent": true, "email": email}, body)

	user, err := storage.FindUserByEmail(email)
	require.NoError(t, err)
	assert.True(t, user.ApprovedAt.IsZero(), "user should not be approved")
	assert.Equal(t, "", user.OrganizationID, "user should not have an organization id")
	assert.Equal(t, "", user.OrganizationName, "user should not have an organization name")
	if assert.Len(t, sentEmails, 1) {
		assert.Equal(t, []string{email}, sentEmails[0].To)
		assert.Contains(t, string(sentEmails[0].Text), "Thanks for your interest")
		assert.Contains(t, string(sentEmails[0].HTML), "Thanks for your interest")
	}

	// Manually approve
	_, err = storage.ApproveUser(user.ID)
	assert.NoError(t, err)
	user, err = storage.FindUserByID(user.ID)
	require.NoError(t, err)
	assert.False(t, user.ApprovedAt.IsZero(), "user should be approved")
	assert.NotEqual(t, "", user.OrganizationID, "user should have an organization id")
	assert.NotEqual(t, "", user.OrganizationName, "user should have an organization name")

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
	u.Path = "/api/users/login" // The frontend will translate /login -> /api/users/login
	w = httptest.NewRecorder()
	r, _ = http.NewRequest("GET", u.String(), nil)
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusFound, w.Code)
	assert.Equal(t, "/org/"+user.OrganizationName, w.HeaderMap.Get("Location"))
	assert.True(t, strings.HasPrefix(w.HeaderMap.Get("Set-Cookie"), cookieName+"="))

	// Invalidates their login token
	user, err = storage.FindUserByEmail(email)
	if assert.NoError(t, err) {
		assert.Equal(t, "", user.Token)
	}
}

func Test_Signup_WithInvalidJSON(t *testing.T) {
	setup(t)
	defer cleanup(t)

	email := ""
	w := httptest.NewRecorder()
	r, _ := http.NewRequest("POST", "/api/users/signup", strings.NewReader("this isn't json"))

	_, err := storage.FindUserByEmail(email)
	assert.EqualError(t, err, ErrNotFound.Error())
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "invalid character 'h' in literal true (expecting 'r')")
	_, err = storage.FindUserByEmail(email)
	assert.EqualError(t, err, ErrNotFound.Error())
}

func Test_Signup_WithBlankEmail(t *testing.T) {
	setup(t)
	defer cleanup(t)

	email := ""
	w := httptest.NewRecorder()
	r, _ := http.NewRequest("POST", "/api/users/signup", jsonBody(t, map[string]interface{}{}))

	_, err := storage.FindUserByEmail(email)
	assert.EqualError(t, err, ErrNotFound.Error())
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Email cannot be blank")
	_, err = storage.FindUserByEmail(email)
	assert.EqualError(t, err, ErrNotFound.Error())
}
