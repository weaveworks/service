package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Lookup(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user := &User{
		ID:    "1",
		Email: "joe@weave.works",
	}
	users[user.Email] = user

	session, err := sessions.Encode(user.ID)
	assert.NoError(t, err)

	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "/users/lookup?session_id="+session, nil)

	Lookup(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func Test_Lookup_NotFound(t *testing.T) {
	setup(t)
	defer cleanup(t)

	session, err := sessions.Encode("foouser")
	assert.NoError(t, err)
	w := httptest.NewRecorder()
	r, _ := http.NewRequest("POST", "/users/lookup?session_id="+session, nil)
	Lookup(w, r)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
