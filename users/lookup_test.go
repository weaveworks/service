package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Lookup(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, err := storage.CreateUser("joe@weave.works")
	assert.NoError(t, err)

	_, err = storage.ApproveUser(user.ID)
	assert.NoError(t, err)
	user, err = storage.FindUserByID(user.ID)
	assert.NoError(t, err)
	assert.NotEqual(t, "", user.Organization.ID)

	cookie, err := sessions.Cookie(user.ID)
	assert.NoError(t, err)

	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "/private/api/users/lookup/"+user.Organization.Name, nil)
	r.AddCookie(cookie)

	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	body := map[string]interface{}{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, map[string]interface{}{"organizationID": user.Organization.ID}, body)
}

func Test_Lookup_NotFound(t *testing.T) {
	setup(t)
	defer cleanup(t)

	cookie, err := sessions.Cookie("foouser")
	assert.NoError(t, err)
	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "/private/api/users/lookup/fooorg", nil)
	r.AddCookie(cookie)
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func Test_PublicLookup(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, err := storage.CreateUser("joe@weave.works")
	require.NoError(t, err)
	user, err = storage.ApproveUser(user.ID)
	require.NoError(t, err)

	cookie, err := sessions.Cookie(user.ID)
	assert.NoError(t, err)

	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "/api/users/lookup", nil)
	r.AddCookie(cookie)

	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	body := map[string]interface{}{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, map[string]interface{}{"organizationName": user.Organization.Name}, body)
}

func Test_PublicLookup_NotFound(t *testing.T) {
	setup(t)
	defer cleanup(t)

	cookie, err := sessions.Cookie("foouser")
	assert.NoError(t, err)

	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "/api/users/lookup", nil)
	r.AddCookie(cookie)

	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func Test_Lookup_ProbeToken(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, err := storage.CreateUser("joe@weave.works")
	require.NoError(t, err)
	user, err = storage.ApproveUser(user.ID)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "/private/api/users/lookup", nil)
	r.Header.Set("Authorization", fmt.Sprintf("Scope-Probe token=%s", user.Organization.ProbeToken))

	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	body := map[string]interface{}{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, map[string]interface{}{"organizationID": user.Organization.ID}, body)

	user, err = storage.FindUserByID(user.ID)
	require.NoError(t, err)
	assert.NotNil(t, user.Organization.FirstProbeUpdateAt)
	assert.False(t, user.Organization.FirstProbeUpdateAt.IsZero())
}
