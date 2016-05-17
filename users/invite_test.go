package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Invite(t *testing.T) {
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
	r, _ := http.NewRequest("POST", "/api/users/org/"+org.Name+"/users", strings.NewReader(`{"email":"fran@weave.works"}`))
	r.AddCookie(cookie)

	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)

	fran, err := storage.FindUserByEmail("fran@weave.works")
	require.NoError(t, err)
	require.Len(t, fran.Organizations, 1)
	assert.Equal(t, org.ID, fran.Organizations[0].ID)

	if assert.Len(t, sentEmails, 1) {
		assert.Equal(t, []string{"fran@weave.works"}, sentEmails[0].To)
		assert.Contains(t, string(sentEmails[0].Text), "You've been invited")
		assert.Contains(t, string(sentEmails[0].HTML), "You've been invited")
	}
}

func Test_Invite_WithInvalidJSON(t *testing.T) {
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
	r, _ := http.NewRequest("POST", "/api/users/org/"+org.Name+"/users", strings.NewReader(`garbage`))
	r.AddCookie(cookie)

	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Len(t, sentEmails, 0)
}

func Test_Invite_WithBlankEmail(t *testing.T) {
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
	r, _ := http.NewRequest("POST", "/api/users/org/"+org.Name+"/users", strings.NewReader(`{"email":""}`))
	r.AddCookie(cookie)

	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Email cannot be blank")
	assert.Len(t, sentEmails, 0)
}

func Test_Invite_UserAlreadyInSameOrganization(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, err := storage.CreateUser("joe@weave.works")
	require.NoError(t, err)

	user, err = storage.ApproveUser(user.ID)
	require.NoError(t, err)

	org, err := storage.CreateOrganization(user.ID)
	require.NoError(t, err)

	fran, err := storage.CreateUser("fran@weave.works")
	require.NoError(t, err)
	fran, err = storage.InviteUser(fran.Email, org.Name)
	require.NoError(t, err)
	require.Len(t, fran.Organizations, 1)
	assert.Equal(t, org.ID, fran.Organizations[0].ID)

	cookie, err := sessions.Cookie(user.ID, "")
	assert.NoError(t, err)

	w := httptest.NewRecorder()
	r, _ := http.NewRequest("POST", "/api/users/org/"+org.Name+"/users", strings.NewReader(`{"email":"fran@weave.works"}`))
	r.AddCookie(cookie)

	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)

	fran, err = storage.FindUserByEmail("fran@weave.works")
	require.NoError(t, err)
	require.Len(t, fran.Organizations, 1)
	assert.Equal(t, org.ID, fran.Organizations[0].ID)

	if assert.Len(t, sentEmails, 1) {
		assert.Equal(t, []string{"fran@weave.works"}, sentEmails[0].To)
		assert.Contains(t, string(sentEmails[0].Text), "You've been invited")
		assert.Contains(t, string(sentEmails[0].HTML), "You've been invited")
	}
}

func Test_Invite_UserNotApproved(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, err := storage.CreateUser("joe@weave.works")
	require.NoError(t, err)

	user, err = storage.ApproveUser(user.ID)
	require.NoError(t, err)

	org, err := storage.CreateOrganization(user.ID)
	require.NoError(t, err)

	fran, err := storage.CreateUser("fran@weave.works")
	require.NoError(t, err)

	cookie, err := sessions.Cookie(user.ID, "")
	assert.NoError(t, err)

	w := httptest.NewRecorder()
	r, _ := http.NewRequest("POST", "/api/users/org/"+org.Name+"/users", strings.NewReader(`{"email":"fran@weave.works"}`))
	r.AddCookie(cookie)

	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)

	fran, err = storage.FindUserByEmail("fran@weave.works")
	require.NoError(t, err)
	require.Len(t, fran.Organizations, 1)
	assert.Equal(t, org.ID, fran.Organizations[0].ID)

	if assert.Len(t, sentEmails, 1) {
		assert.Equal(t, []string{"fran@weave.works"}, sentEmails[0].To)
		assert.Contains(t, string(sentEmails[0].Text), "You've been invited")
		assert.Contains(t, string(sentEmails[0].HTML), "You've been invited")
	}
}

func Test_Invite_UserInDifferentOrganization(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, err := storage.CreateUser("joe@weave.works")
	require.NoError(t, err)

	user, err = storage.ApproveUser(user.ID)
	require.NoError(t, err)

	org, err := storage.CreateOrganization(user.ID)
	require.NoError(t, err)

	fran, err := storage.CreateUser("fran@weave.works")
	require.NoError(t, err)
	fran, err = storage.ApproveUser(fran.ID)
	require.NoError(t, err)
	franOrg, err := storage.CreateOrganization(fran.ID)
	require.NoError(t, err)

	cookie, err := sessions.Cookie(user.ID, "")
	assert.NoError(t, err)

	w := httptest.NewRecorder()
	r, _ := http.NewRequest("POST", "/api/users/org/"+org.Name+"/users", strings.NewReader(`{"email":"fran@weave.works"}`))
	r.AddCookie(cookie)

	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), `{"errors":[{"message":"Email is already taken"}]}`)

	fran, err = storage.FindUserByEmail("fran@weave.works")
	require.NoError(t, err)
	require.Len(t, fran.Organizations, 1)
	assert.Equal(t, franOrg.ID, fran.Organizations[0].ID)
	assert.Len(t, sentEmails, 0)
}
