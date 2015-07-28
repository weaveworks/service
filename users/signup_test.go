package main

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

func Test_Signup(t *testing.T) {
	setup(t)
	defer cleanup(t)

	email := "joe@weave.works"

	// Signup as a new user, should send welcome email
	w := httptest.NewRecorder()
	r, _ := http.NewRequest("POST", "/api/users/signup?email=joe%40weave.works", nil)
	_, err := storage.FindUserByEmail(email)
	assert.EqualError(t, err, ErrNotFound.Error())
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	body := map[string]interface{}{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, map[string]interface{}{"mailSent": true, "email": email}, body)

	user, err := storage.FindUserByEmail(email)
	if !assert.NoError(t, err) {
		return
	}
	organizationID := user.OrganizationID
	organizationName := user.OrganizationName
	assert.True(t, user.ApprovedAt.IsZero())
	assert.Equal(t, "", organizationID)
	assert.Equal(t, "", organizationName)
	if assert.Len(t, sentEmails, 1) {
		assert.Equal(t, []string{email}, sentEmails[0].To)
		assert.Contains(t, string(sentEmails[0].Text), "Thanks for your interest")
		assert.Contains(t, string(sentEmails[0].HTML), "Thanks for your interest")
	}

	// Manually approve
	assert.NoError(t, storage.ApproveUser(user.ID))
	organizationName = user.OrganizationName
	assert.False(t, user.ApprovedAt.IsZero())
	assert.NotEqual(t, "", user.OrganizationID)
	assert.NotEqual(t, "", user.OrganizationName)

	// Do it again: check it preserves their data, and sends a login email
	w = httptest.NewRecorder()
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	body = map[string]interface{}{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, map[string]interface{}{"mailSent": true, "email": email}, body)
	if !assert.Len(t, sentEmails, 2) {
		return
	}
	user, err = storage.FindUserByEmail(email)
	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, []string{email}, sentEmails[1].To)
	loginLink, emailToken := findLoginLink(t, sentEmails[1])
	assert.Contains(t, string(sentEmails[1].HTML), loginLink)

	// Check the db one was hashed
	dbToken := user.Token
	assert.NotEqual(t, "", dbToken)
	assert.NotEqual(t, dbToken, emailToken)

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
	assert.Equal(t, "/api/users/org/"+organizationName, w.HeaderMap.Get("Location"))
	assert.True(t, strings.HasPrefix(w.HeaderMap.Get("Set-Cookie"), cookieName+"="))

	// Invalidates their login token
	user, err = storage.FindUserByEmail(email)
	if assert.NoError(t, err) {
		assert.Equal(t, "", user.Token)
	}
}

func Test_Signup_WithBlankEmail(t *testing.T) {
	setup(t)
	defer cleanup(t)

	email := ""
	w := httptest.NewRecorder()
	r, _ := http.NewRequest("POST", "/api/users/signup", nil)

	_, err := storage.FindUserByEmail(email)
	assert.EqualError(t, err, ErrNotFound.Error())
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Email cannot be blank")
	_, err = storage.FindUserByEmail(email)
	assert.EqualError(t, err, ErrNotFound.Error())
}
