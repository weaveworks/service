package main

import (
	"encoding/json"
	"fmt"
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

	cookie, err := sessions.Cookie(user.ID, "")
	assert.NoError(t, err)

	// Create the user's first organization
	name, err := storage.GenerateOrganizationName()
	require.NoError(t, err)
	org, err := storage.CreateOrganization(user.ID, name, name)
	assert.NoError(t, err)
	assert.NotEqual(t, "", org.ID)
	assert.NotEqual(t, "", org.Name)
	assert.Equal(t, org.Name, org.Label)

	// Check the user was added to the org
	user, err = storage.FindUserByID(user.ID)
	assert.NoError(t, err)
	require.Len(t, user.Organizations, 1)
	assert.Equal(t, org.ID, user.Organizations[0].ID, "user should have an organization id")
	assert.Equal(t, org.Name, user.Organizations[0].Name, "user should have an organization name")
	assert.Equal(t, org.Label, user.Organizations[0].Label, "user should have an organization label")
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
		"label":              org.Label,
		"probeToken":         org.ProbeToken,
		"firstProbeUpdateAt": org.FirstProbeUpdateAt.UTC().Format(time.RFC3339),
	}, body)
}

func Test_Org_NoProbeUpdates(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, err := storage.CreateUser("joe@weave.works")
	require.NoError(t, err)

	user, err = storage.ApproveUser(user.ID)
	require.NoError(t, err)

	name, err := storage.GenerateOrganizationName()
	require.NoError(t, err)
	org, err := storage.CreateOrganization(user.ID, name, name)
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
		"label":      org.Label,
		"probeToken": org.ProbeToken,
	}, body)
}

func Test_ListOrganizationUsers(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, err := storage.CreateUser("joe@weave.works")
	require.NoError(t, err)

	user, err = storage.ApproveUser(user.ID)
	require.NoError(t, err)

	name, err := storage.GenerateOrganizationName()
	require.NoError(t, err)
	org, err := storage.CreateOrganization(user.ID, name, name)
	require.NoError(t, err)

	fran, err := storage.CreateUser("fran@weave.works")
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
	assert.Contains(t, w.Body.String(), `{"users":[{"email":"joe@weave.works","self":true},{"email":"fran@weave.works"}]}`)
}

func Test_RelabelOrganization(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, err := storage.CreateUser("joe@weave.works")
	require.NoError(t, err)

	user, err = storage.ApproveUser(user.ID)
	require.NoError(t, err)

	name, err := storage.GenerateOrganizationName()
	require.NoError(t, err)
	org, err := storage.CreateOrganization(user.ID, name, name)
	require.NoError(t, err)

	cookie, err := sessions.Cookie(user.ID, "")
	assert.NoError(t, err)

	w := httptest.NewRecorder()
	r, _ := http.NewRequest("PUT", "/api/users/org/"+org.Name, strings.NewReader(`{"label":"my-organization"}`))
	r.AddCookie(cookie)

	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)

	user, err = storage.FindUserByID(user.ID)
	require.NoError(t, err)
	if assert.Len(t, user.Organizations, 1) {
		assert.Equal(t, org.ID, user.Organizations[0].ID)
		assert.Equal(t, org.Name, user.Organizations[0].Name)
		assert.Equal(t, "my-organization", user.Organizations[0].Label)
	}
}

func Test_RenameOrganization_NotAllowed(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, err := storage.CreateUser("joe@weave.works")
	require.NoError(t, err)

	user, err = storage.ApproveUser(user.ID)
	require.NoError(t, err)

	name, err := storage.GenerateOrganizationName()
	require.NoError(t, err)
	org, err := storage.CreateOrganization(user.ID, name, name)
	require.NoError(t, err)

	cookie, err := sessions.Cookie(user.ID, "")
	assert.NoError(t, err)

	w := httptest.NewRecorder()
	r, _ := http.NewRequest("PUT", "/api/users/org/"+org.Name, strings.NewReader(`{"name":"my-organization"}`))
	r.AddCookie(cookie)

	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), `{"errors":[{"message":"Name cannot be changed"}]}`)

	user, err = storage.FindUserByID(user.ID)
	require.NoError(t, err)
	if assert.Len(t, user.Organizations, 1) {
		assert.Equal(t, org.ID, user.Organizations[0].ID)
		assert.Equal(t, org.Name, user.Organizations[0].Name)
		assert.Equal(t, org.Label, user.Organizations[0].Label)
	}
}

func Test_RelabelOrganization_Validation(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, err := storage.CreateUser("joe@weave.works")
	require.NoError(t, err)

	user, err = storage.ApproveUser(user.ID)
	require.NoError(t, err)

	name, err := storage.GenerateOrganizationName()
	require.NoError(t, err)
	org, err := storage.CreateOrganization(user.ID, name, name)
	require.NoError(t, err)

	cookie, err := sessions.Cookie(user.ID, "")
	assert.NoError(t, err)

	for label, errMsg := range map[string]string{
		"": "Label cannot be blank",
	} {
		w := httptest.NewRecorder()
		r, _ := http.NewRequest("PUT", "/api/users/org/"+org.Name, strings.NewReader(fmt.Sprintf(`{"label":%q}`, label)))
		r.AddCookie(cookie)

		app.ServeHTTP(w, r)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), fmt.Sprintf(`{"errors":[{"message":%q}]}`, errMsg))

		user, err = storage.FindUserByID(user.ID)
		require.NoError(t, err)
		if assert.Len(t, user.Organizations, 1) {
			assert.Equal(t, org.ID, user.Organizations[0].ID)
			assert.Equal(t, org.Name, user.Organizations[0].Name)
			assert.Equal(t, org.Label, user.Organizations[0].Label)
		}
	}
}

func Test_CustomNameOrganization(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, err := storage.CreateUser("joe@weave.works")
	require.NoError(t, err)

	user, err = storage.ApproveUser(user.ID)
	require.NoError(t, err)

	cookie, err := sessions.Cookie(user.ID, "")
	assert.NoError(t, err)

	w := httptest.NewRecorder()
	r, _ := http.NewRequest("POST", "/api/users/org", strings.NewReader(`{"name":"my-organization","label":"my organization"}`))
	r.AddCookie(cookie)

	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusCreated, w.Code)

	user, err = storage.FindUserByID(user.ID)
	require.NoError(t, err)
	if assert.Len(t, user.Organizations, 1) {
		assert.NotEqual(t, "", user.Organizations[0].ID)
		assert.Equal(t, "my-organization", user.Organizations[0].Name)
		assert.Equal(t, "my organization", user.Organizations[0].Label)
	}
}

func Test_CustomNameOrganization_Validation(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, err := storage.CreateUser("joe@weave.works")
	require.NoError(t, err)

	user, err = storage.ApproveUser(user.ID)
	require.NoError(t, err)

	name, err := storage.GenerateOrganizationName()
	require.NoError(t, err)
	otherOrg, err := storage.CreateOrganization(user.ID, name, name)
	require.NoError(t, err)

	cookie, err := sessions.Cookie(user.ID, "")
	assert.NoError(t, err)

	for name, errMsg := range map[string]string{
		"": "Name cannot be blank",
		"org with^/invalid&characters": "Name can only contain letters, numbers, hyphen, and underscore",
		otherOrg.Name:                  "Name is already taken",
	} {
		w := httptest.NewRecorder()
		r, _ := http.NewRequest("POST", "/api/users/org", strings.NewReader(fmt.Sprintf(`{"name":%q,"label":"my organization"}`, name)))
		r.AddCookie(cookie)

		app.ServeHTTP(w, r)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), fmt.Sprintf(`{"errors":[{"message":%q}]}`, errMsg))

		user, err = storage.FindUserByID(user.ID)
		require.NoError(t, err)
		if assert.Len(t, user.Organizations, 1) {
			assert.Equal(t, otherOrg.ID, user.Organizations[0].ID)
			assert.Equal(t, otherOrg.Name, user.Organizations[0].Name)
		}
	}
}

func Test_Organization_CheckIfNameExists(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, err := storage.CreateUser("joe@weave.works")
	require.NoError(t, err)

	user, err = storage.ApproveUser(user.ID)
	require.NoError(t, err)

	otherUser, err := storage.CreateUser("otherUser@weave.works")
	require.NoError(t, err)

	name, err := storage.GenerateOrganizationName()
	require.NoError(t, err)

	cookie, err := sessions.Cookie(user.ID, "")
	assert.NoError(t, err)

	r, _ := http.NewRequest("GET", "/api/users/org/"+name, nil)
	r.AddCookie(cookie)

	{
		w := httptest.NewRecorder()
		app.ServeHTTP(w, r)
		assert.Equal(t, http.StatusNotFound, w.Code)
	}

	_, err = storage.CreateOrganization(otherUser.ID, name, name)
	require.NoError(t, err)

	{
		w := httptest.NewRecorder()
		app.ServeHTTP(w, r)
		assert.Equal(t, http.StatusForbidden, w.Code)
	}
}
