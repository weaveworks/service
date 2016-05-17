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

	user, err := storage.CreateUser("", "joe@weave.works")
	assert.NoError(t, err)

	user, err = storage.ApproveUser(user.ID)
	require.NoError(t, err)

	cookie, err := sessions.Cookie(user.ID, "")
	assert.NoError(t, err)

	// Create the user's first organization
	org, err := storage.CreateOrganization(user.ID)
	assert.NoError(t, err)

	// Check the user was added to the org
	user, err = storage.FindUserByID(user.ID)
	assert.NoError(t, err)
	require.Len(t, user.Organizations, 1)
	assert.Equal(t, org.Name, user.Organizations[0].Name, "user should have an organization name")
	assert.NotEqual(t, "", user.Organizations[0].ID, "user should have an organization id")
	assert.NotEqual(t, "", user.Organizations[0].ProbeToken, "user should have a probe token")

	org, err = storage.FindOrganizationByProbeToken(user.Organizations[0].ProbeToken)
	require.NoError(t, err)
	require.NotNil(t, org.FirstProbeUpdateAt)

	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "/api/users/org/"+user.Organizations[0].Name, nil)
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

	user, err := storage.CreateUser("", "joe@weave.works")
	require.NoError(t, err)

	user, err = storage.ApproveUser(user.ID)
	require.NoError(t, err)

	org, err := storage.CreateOrganization(user.ID)
	require.NoError(t, err)

	cookie, err := sessions.Cookie(user.ID, "")
	assert.NoError(t, err)

	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "/api/users/org/"+org.Name, nil)
	r.AddCookie(cookie)

	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	body := map[string]interface{}{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, map[string]interface{}{
		"user":       user.Email,
		"name":       org.Name,
		"probeToken": org.ProbeToken,
	}, body)
}

func Test_ListOrganizationUsers(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, err := storage.CreateUser("", "joe@weave.works")
	require.NoError(t, err)

	user, err = storage.ApproveUser(user.ID)
	require.NoError(t, err)

	org, err := storage.CreateOrganization(user.ID)
	require.NoError(t, err)

	fran, err := storage.CreateUser("", "fran@weave.works")
	require.NoError(t, err)
	fran, err = storage.InviteUser(fran.Email, org.Name)
	require.NoError(t, err)

	cookie, err := sessions.Cookie(user.ID, "")
	assert.NoError(t, err)

	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "/api/users/org/"+org.Name+"/users", nil)
	r.AddCookie(cookie)

	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `[{"email":"joe@weave.works","self":true},{"email":"fran@weave.works"}]`)
}

func Test_RenameOrganization(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, err := storage.CreateUser("", "joe@weave.works")
	require.NoError(t, err)

	user, err = storage.ApproveUser(user.ID)
	require.NoError(t, err)

	org, err := storage.CreateOrganization(user.ID)
	require.NoError(t, err)

	orgID := org.ID
	assert.NotEqual(t, "", orgID)
	orgName := org.Name
	assert.NotEqual(t, "", orgName)

	cookie, err := sessions.Cookie(user.ID, "")
	assert.NoError(t, err)

	w := httptest.NewRecorder()
	r, _ := http.NewRequest("PUT", "/api/users/org/"+orgName, strings.NewReader(`{"name":"my-organization"}`))
	r.AddCookie(cookie)

	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)

	user, err = storage.FindUserByID(user.ID)
	require.NoError(t, err)
	if assert.Len(t, user.Organizations, 1) {
		assert.Equal(t, orgID, user.Organizations[0].ID)
		assert.Equal(t, "my-organization", user.Organizations[0].Name)
	}
}

func Test_RenameOrganization_Validation(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, err := storage.CreateUser("", "joe@weave.works")
	require.NoError(t, err)

	user, err = storage.ApproveUser(user.ID)
	require.NoError(t, err)

	org, err := storage.CreateOrganization(user.ID)
	require.NoError(t, err)

	orgID := org.ID
	assert.NotEqual(t, "", orgID)
	orgName := org.Name
	assert.NotEqual(t, "", orgName)

	cookie, err := sessions.Cookie(user.ID, "")
	assert.NoError(t, err)

	w := httptest.NewRecorder()
	r, _ := http.NewRequest("PUT", "/api/users/org/"+orgName, strings.NewReader(`{"name":"org with^/invalid&characters"}`))
	r.AddCookie(cookie)

	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), `{"errors":[{"message":"Name can only contain letters, numbers, hyphen, and underscore"}]}`)

	user, err = storage.FindUserByID(user.ID)
	require.NoError(t, err)
	if assert.Len(t, user.Organizations, 1) {
		assert.Equal(t, orgID, user.Organizations[0].ID)
		assert.Equal(t, orgName, user.Organizations[0].Name)
	}
}
