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
)

func findLoginLink(t *testing.T, e *email.Email) (url, token string, ok bool) {
	pattern := `http://` + domain + `/users/signup\?email=[\w.%]+&token=([A-Za-z0-9%.]+)`
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(string(e.Text))
	if assert.Len(t, matches, 2, fmt.Sprintf("Could not find Login Link in text: %q", e.Text)) &&
		assert.NotEqual(t, "", matches[0]) &&
		assert.NotEqual(t, "", matches[1]) &&
		assert.Contains(t, string(e.HTML), matches[0], fmt.Sprintf("Could not find Login Link in html: %q", e.HTML)) {
		return matches[0], matches[1], true
	}
	return "", "", false
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
	appName := users[email].AppName
	assert.NotEqual(t, "", appName)
	if assert.Len(t, sentEmails, 1) {
		assert.Equal(t, []string{email}, sentEmails[0].To)
		assert.Contains(t, string(sentEmails[0].Text), "Thanks for your interest")
		assert.Contains(t, string(sentEmails[0].HTML), "Thanks for your interest")
	}

	// Manually approve
	users[email].ApprovedAt = time.Now()

	// Do it again: check it preserves their data, and sends a login email
	Signup(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Login email sent")
	if !assert.NotNil(t, users[email]) {
		return
	}
	assert.Equal(t, appName, users[email].AppName)
	if !assert.Len(t, sentEmails, 2) {
		return
	}
	assert.Equal(t, []string{email}, sentEmails[1].To)
	loginLink, emailToken, ok := findLoginLink(t, sentEmails[1])
	if !ok {
		return
	}
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
	assert.Equal(t, "/app/"+appName, w.HeaderMap.Get("Location"))
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
