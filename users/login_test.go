package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Login_NoParams(t *testing.T) {
	setup(t)
	defer cleanup(t)

	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "/api/users/login", nil)
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), `{"errors":[{"message":"Email cannot be blank"}]}`)
}

func Test_Login_Unauthorized(t *testing.T) {
	setup(t)
	defer cleanup(t)

	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "/api/users/login?email=joe@weave.works&token=foo", nil)
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), `{"errors":[{"message":"Invalid authentication data"}]}`)
}
