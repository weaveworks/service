package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

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
	token := users[email].Token
	assert.NotEqual(t, "", token)
	loginLink := "http://example.com/users/signup?email=joe%40weave.works&token=" + token
	if assert.Len(t, sentEmails, 2) {
		assert.Equal(t, []string{email}, sentEmails[1].To)
		assert.Contains(t, string(sentEmails[1].Text), loginLink)
		assert.Contains(t, string(sentEmails[1].HTML), loginLink)
	}

	// Login with the token
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
