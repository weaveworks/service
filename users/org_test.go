package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

func Test_RenameOrganization(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, err := storage.CreateUser("joe@weave.works")
	assert.NoError(t, err)
	user, err = storage.ApproveUser(user.ID)
	require.NoError(t, err)
	orgID := user.Organization.ID
	assert.NotEqual(t, "", orgID)
	orgName := user.Organization.Name
	assert.NotEqual(t, "", orgName)

	cookie, err := sessions.Cookie(user.ID)
	assert.NoError(t, err)

	w := httptest.NewRecorder()
	r, _ := http.NewRequest("PUT", "/api/users/org/"+orgName, strings.NewReader(`{"name":"my-organization"}`))
	r.AddCookie(cookie)

	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)

	user, err = storage.FindUserByID(user.ID)
	require.NoError(t, err)
	assert.Equal(t, orgID, user.Organization.ID)
	assert.Equal(t, "my-organization", user.Organization.Name)
}

func Test_RenameOrganization_Validation(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, err := storage.CreateUser("joe@weave.works")
	assert.NoError(t, err)
	user, err = storage.ApproveUser(user.ID)
	require.NoError(t, err)
	orgID := user.Organization.ID
	assert.NotEqual(t, "", orgID)
	orgName := user.Organization.Name
	assert.NotEqual(t, "", orgName)

	cookie, err := sessions.Cookie(user.ID)
	assert.NoError(t, err)

	w := httptest.NewRecorder()
	r, _ := http.NewRequest("PUT", "/api/users/org/"+orgName, strings.NewReader(`{"name":"org with^/invalid&characters"}`))
	r.AddCookie(cookie)

	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), `{"errors":[{"message":"Name can only contain letters, numbers, hyphen, and underscore"}]}`)

	user, err = storage.FindUserByID(user.ID)
	require.NoError(t, err)
	assert.Equal(t, orgID, user.Organization.ID)
	assert.Equal(t, orgName, user.Organization.Name)
}
