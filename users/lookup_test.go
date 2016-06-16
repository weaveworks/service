package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Lookup(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, err := storage.CreateUser("joe@weave.works")
	require.NoError(t, err)
	user, err = storage.ApproveUser(user.ID)
	require.NoError(t, err)
	org, err := storage.CreateOrganization(user.ID)
	require.NoError(t, err)

	cookie, err := sessions.Cookie(user.ID, "")
	assert.NoError(t, err)

	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "/private/api/users/lookup/"+org.Name, nil)
	r.AddCookie(cookie)

	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	body := map[string]interface{}{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, map[string]interface{}{"organizationID": org.ID}, body)
}

func Test_Lookup_NotFound(t *testing.T) {
	setup(t)
	defer cleanup(t)

	cookie, err := sessions.Cookie("foouser", "")
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
	org, err := storage.CreateOrganization(user.ID)
	require.NoError(t, err)

	org, err = storage.FindOrganizationByProbeToken(org.ProbeToken)
	require.NoError(t, err)
	require.NotNil(t, org.FirstProbeUpdateAt)

	cookie, err := sessions.Cookie(user.ID, "")
	assert.NoError(t, err)
	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "/api/users/lookup", nil)
	r.AddCookie(cookie)

	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	body := map[string]interface{}{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, user.Email, body["email"])
	assert.Equal(t, map[string]interface{}{
		"email": user.Email,
		"organizations": []interface{}{
			map[string]interface{}{
				"name":               org.Name,
				"firstProbeUpdateAt": org.FirstProbeUpdateAt.UTC().Format(time.RFC3339),
			},
		},
	}, body)
}

func Test_PublicLookup_NotFound(t *testing.T) {
	setup(t)
	defer cleanup(t)

	cookie, err := sessions.Cookie("foouser", "")
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
	org, err := storage.CreateOrganization(user.ID)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "/private/api/users/lookup", nil)
	r.Header.Set("Authorization", fmt.Sprintf("Scope-Probe token=%s", org.ProbeToken))

	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	body := map[string]interface{}{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, map[string]interface{}{"organizationID": org.ID}, body)

	user, err = storage.FindUserByID(user.ID)
	require.NoError(t, err)
	require.Len(t, user.Organizations, 1)
	assert.NotNil(t, user.Organizations[0].FirstProbeUpdateAt)
	assert.False(t, user.Organizations[0].FirstProbeUpdateAt.IsZero())
}

func Test_Lookup_Admin(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, err := storage.CreateUser("joe@weave.works")
	require.NoError(t, err)
	user, err = storage.ApproveUser(user.ID)
	require.NoError(t, err)
	require.NoError(t, storage.SetUserAdmin(user.ID, true))

	cookie, err := sessions.Cookie(user.ID, "")
	assert.NoError(t, err)

	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "/private/api/users/admin", nil)
	r.AddCookie(cookie)

	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	body := map[string]interface{}{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, map[string]interface{}{"adminID": user.ID}, body)
}

func Test_Lookup_Admin_Unauthorized(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, err := storage.CreateUser("joe@weave.works")
	require.NoError(t, err)
	user, err = storage.ApproveUser(user.ID)
	require.NoError(t, err)

	cookie, err := sessions.Cookie(user.ID, "")
	assert.NoError(t, err)

	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "/private/api/users/admin", nil)
	r.AddCookie(cookie)

	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
