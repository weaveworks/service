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
	assert.NotEqual(t, "", user.OrganizationID)

	cookie, err := sessions.Cookie(user.ID)
	assert.NoError(t, err)

	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "/private/api/users/lookup/"+user.OrganizationName, nil)
	r.AddCookie(cookie)

	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	body := map[string]interface{}{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, map[string]interface{}{"organizationID": user.OrganizationID}, body)
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
	assert.Len(t, w.Body.Bytes(), 0)
}

func Test_Lookup_ProbeToken(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, err := storage.CreateUser("joe@weave.works")
	require.NoError(t, err)

	_, err = storage.ApproveUser(user.ID)
	require.NoError(t, err)
	user, err = storage.FindUserByID(user.ID)
	require.NoError(t, err)
	assert.NotEqual(t, "", user.OrganizationID)
	assert.NotEqual(t, "", user.ProbeToken)

	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "/private/api/users/lookup/"+user.OrganizationName, nil)
	r.Header.Set("Authorization", fmt.Sprintf("Probe %s", user.ProbeToken))

	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	body := map[string]interface{}{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, map[string]interface{}{"organizationID": user.OrganizationID}, body)
}
