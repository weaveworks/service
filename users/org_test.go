package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Org(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, err := storage.CreateUser("joe@weave.works")
	assert.NoError(t, err)

	user, err = storage.ApproveUser(user.ID)
	require.NoError(t, err)
	assert.NotEqual(t, "", user.Organization.ID)

	org, err := storage.FindOrganizationByProbeToken(user.Organization.ProbeToken)
	require.NoError(t, err)
	require.NotNil(t, org.FirstProbeUpdateAt)

	cookie, err := sessions.Cookie(user.ID)
	assert.NoError(t, err)

	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "/api/users/org/"+user.Organization.Name, nil)
	r.AddCookie(cookie)

	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	body := map[string]interface{}{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, map[string]interface{}{
		"user":               user.Email,
		"name":               org.Name,
		"probeToken":         org.ProbeToken,
		"firstProbeUpdateAt": org.FirstProbeUpdateAt.UTC().Format(time.RFC3339),
	}, body)
}

func Test_Org_NoProbeUpdates(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, err := storage.CreateUser("joe@weave.works")
	assert.NoError(t, err)

	user, err = storage.ApproveUser(user.ID)
	require.NoError(t, err)
	assert.NotEqual(t, "", user.Organization.ID)

	cookie, err := sessions.Cookie(user.ID)
	assert.NoError(t, err)

	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "/api/users/org/"+user.Organization.Name, nil)
	r.AddCookie(cookie)

	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	body := map[string]interface{}{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, map[string]interface{}{
		"user":       user.Email,
		"name":       user.Organization.Name,
		"probeToken": user.Organization.ProbeToken,
	}, body)
}

func Test_ListOrganizationUsers(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, err := storage.CreateUser("joe@weave.works")
	assert.NoError(t, err)

	user, err = storage.ApproveUser(user.ID)
	require.NoError(t, err)
	assert.NotEqual(t, "", user.Organization.ID)

	cookie, err := sessions.Cookie(user.ID)
	assert.NoError(t, err)

	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "/api/users/org/"+user.Organization.Name+"/users", nil)
	r.AddCookie(cookie)

	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `[{"email":"joe@weave.works"}]`)
}
