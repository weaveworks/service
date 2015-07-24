package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jordan-wright/email"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func findLoginLink(t *testing.T, e *email.Email) (url, token string) {
	pattern := `http://` + domain + `/users/signup\?email=[\w.%]+&token=([A-Za-z0-9%.]+)`
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
	r, _ := http.NewRequest("POST", "/users/signup?email=joe%40weave.works", nil)
	assert.Nil(t, users[email])
	Signup(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Login email sent")
	if !assert.NotNil(t, users[email]) {
		return
	}
	user := users[email]
	organizationID := user.OrganizationID
	organizationName := user.OrganizationName
	assert.NotEqual(t, "", user.OrganizationID)
	assert.NotEqual(t, "", user.OrganizationName)
	if assert.Len(t, sentEmails, 1) {
		assert.Equal(t, []string{email}, sentEmails[0].To)
		assert.Contains(t, string(sentEmails[0].Text), "Thanks for your interest")
		assert.Contains(t, string(sentEmails[0].HTML), "Thanks for your interest")
	}

	// Manually approve
	user = users[email]
	require.NotNil(t, user)
	user.ApprovedAt = time.Now()
	organizationID = user.OrganizationID
	organizationName = user.OrganizationName
	assert.NotEqual(t, "", organizationID)
	assert.NotEqual(t, "", organizationName)

	// Do it again: check it preserves their data, and sends a login email
	Signup(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Login email sent")
	if !assert.NotNil(t, users[email]) {
		return
	}
	assert.Equal(t, organizationID, users[email].OrganizationID)
	assert.Equal(t, organizationName, users[email].OrganizationName)
	if !assert.Len(t, sentEmails, 2) {
		return
	}
	assert.Equal(t, []string{email}, sentEmails[1].To)
	loginLink, emailToken := findLoginLink(t, sentEmails[1])
	assert.Contains(t, string(sentEmails[1].HTML), loginLink)

	// Check the db one was hashed
	dbToken := users[email].Token
	assert.NotEqual(t, "", dbToken)
	assert.NotEqual(t, dbToken, emailToken)

	// Check the email one wasn't hashed (by looking for dollar-signs)
	assert.NotContains(t, emailToken, "$")
	assert.NotContains(t, emailToken, "%24")

	// Login with the link
	w = httptest.NewRecorder()
	r, _ = http.NewRequest("GET", loginLink, nil)
	Signup(w, r)
	assert.Equal(t, http.StatusFound, w.Code)
	assert.Equal(t, "/app/"+organizationName, w.HeaderMap.Get("Location"))
	assert.True(t, strings.HasPrefix(w.HeaderMap.Get("Set-Cookie"), cookieName+"="))

	// Invalidates their login token
	assert.Equal(t, "", users[email].Token)
}

func Test_Signup_WithBlankEmail(t *testing.T) {
	setup(t)
	defer cleanup(t)

	email := ""
	w := httptest.NewRecorder()
	r, _ := http.NewRequest("POST", "/users/signup", nil)

	assert.Nil(t, users[email])
	Signup(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Email cannot be blank")
	assert.Nil(t, users[email])
}
