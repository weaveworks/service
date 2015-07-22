package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_Lookup(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user := &User{
		Email:         "joe@weave.works",
		SessionID:     "session1234",
		SessionExpiry: time.Now().UTC().Add(1 * time.Hour),
	}
	users[user.Email] = user

	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "/users/lookup?session_id=session1234", nil)

	Lookup(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func Test_Lookup_NotFound(t *testing.T) {
	setup(t)
	defer cleanup(t)

	w := httptest.NewRecorder()
	r, _ := http.NewRequest("POST", "/users/lookup?session_id=foobar", nil)
	Lookup(w, r)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
