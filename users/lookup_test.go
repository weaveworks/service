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

	user, err := storage.CreateUser("joe@weave.works")
	assert.NoError(t, err)

	session, err := sessions.Encode(user.ID)
	assert.NoError(t, err)

	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "/api/users/private/lookup?session_id="+session, nil)

	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func Test_Lookup_NotFound(t *testing.T) {
	setup(t)
	defer cleanup(t)

	session, err := sessions.Encode("foouser")
	assert.NoError(t, err)
	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "/api/users/private/lookup?session_id="+session, nil)
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
