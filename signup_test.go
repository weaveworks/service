package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Signup(t *testing.T) {
	setup(t)
	defer cleanup(t)

	email := "joe@weave.works"
	w := httptest.NewRecorder()
	r, _ := http.NewRequest(
		"POST",
		"/users/signup?email=joe%40weave.works",
		nil,
	)

	assert.Nil(t, users[email])
	Signup(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Login email sent")
	if !assert.NotNil(t, users[email]) {
		return
	}
	appName := users[email].AppName
	assert.NotEqual(t, "", appName)

	// Do it again, and check it preserves their data
	Signup(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Login email sent")
	if !assert.NotNil(t, users[email]) {
		return
	}
	assert.Equal(t, appName, users[email].AppName)

	// Login with the token
	token := users[email].Token
	assert.NotEqual(t, "", token)

	w = httptest.NewRecorder()
	r, _ = http.NewRequest("GET", "/users/signup?token="+token, nil)

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
