package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_InviteNonExistentUser(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, org := getOrg(t)
	franEmail := "fran@weave.works"

	w := httptest.NewRecorder()
	r := requestAs(t, user, "POST", "/api/users/org/"+org.ExternalID+"/users", jsonBody{"email": franEmail}.Reader(t))

	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	body := map[string]interface{}{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, map[string]interface{}{
		"mailSent": true,
		"email":    franEmail,
	}, body)

	fran, err := storage.FindUserByEmail(franEmail)
	require.NoError(t, err)
	require.Len(t, fran.Organizations, 1)
	assert.Equal(t, org.ID, fran.Organizations[0].ID)
	assert.Equal(t, fran.Email, franEmail)

	if assert.Len(t, sentEmails, 1) {
		assert.Equal(t, []string{franEmail}, sentEmails[0].To)
		assert.Contains(t, string(sentEmails[0].Text), "has invited you")
		assert.Contains(t, string(sentEmails[0].HTML), "has invited you")
	}
}

func Test_InviteExistingUser(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, org := getOrg(t)
	fran := getApprovedUser(t)

	w := httptest.NewRecorder()
	r := requestAs(t, user, "POST", "/api/users/org/"+org.ExternalID+"/users", jsonBody{"email": fran.Email}.Reader(t))

	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	body := map[string]interface{}{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, map[string]interface{}{
		"mailSent": true,
		"email":    fran.Email,
	}, body)

	fran, err := storage.FindUserByEmail(fran.Email)
	require.NoError(t, err)
	require.Len(t, fran.Organizations, 1)
	assert.Equal(t, org.ID, fran.Organizations[0].ID)

	if assert.Len(t, sentEmails, 1) {
		assert.Equal(t, []string{fran.Email}, sentEmails[0].To)
		assert.Contains(t, string(sentEmails[0].Text), "has granted you access")
		assert.Contains(t, string(sentEmails[0].HTML), "has granted you access")
	}
}

func Test_Invite_WithInvalidJSON(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, org := getOrg(t)

	w := httptest.NewRecorder()
	r := requestAs(t, user, "POST", "/api/users/org/"+org.ExternalID+"/users", strings.NewReader(`garbage`))

	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Len(t, sentEmails, 0)
}

func Test_Invite_WithBlankEmail(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, org := getOrg(t)

	w := httptest.NewRecorder()
	r := requestAs(t, user, "POST", "/api/users/org/"+org.ExternalID+"/users", jsonBody{"email": ""}.Reader(t))

	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Email cannot be blank")
	assert.Len(t, sentEmails, 0)
}

func Test_Invite_UserAlreadyInSameOrganization(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, org := getOrg(t)

	fran, err := storage.CreateUser("fran@weave.works")
	require.NoError(t, err)
	fran, created, err := storage.InviteUser(fran.Email, org.ExternalID)
	require.NoError(t, err)
	require.Len(t, fran.Organizations, 1)
	assert.Equal(t, org.ID, fran.Organizations[0].ID)
	assert.Equal(t, created, false)

	w := httptest.NewRecorder()
	r := requestAs(t, user, "POST", "/api/users/org/"+org.ExternalID+"/users", jsonBody{"email": fran.Email}.Reader(t))

	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)

	fran, err = storage.FindUserByEmail(fran.Email)
	require.NoError(t, err)
	require.Len(t, fran.Organizations, 1)
	assert.Equal(t, org.ID, fran.Organizations[0].ID)

	if assert.Len(t, sentEmails, 1) {
		assert.Equal(t, []string{"fran@weave.works"}, sentEmails[0].To)
		assert.Contains(t, string(sentEmails[0].Text), "has granted you access")
		assert.Contains(t, string(sentEmails[0].HTML), "has granted you access")
	}
}

func Test_Invite_UserToAnOrgIDontOwn(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user := getApprovedUser(t)
	otherUser := getApprovedUser(t)
	_, otherOrg := getOrg(t)

	w := httptest.NewRecorder()
	r := requestAs(t, user, "POST", "/api/users/org/"+otherOrg.ExternalID+"/users", jsonBody{"email": otherUser.Email}.Reader(t))

	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusForbidden, w.Code)

	otherUser, err := storage.FindUserByID(otherUser.ID)
	require.NoError(t, err)
	require.Len(t, otherUser.Organizations, 0)
	assert.Len(t, sentEmails, 0)
}

func Test_Invite_UserNotApproved(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, org := getOrg(t)

	fran, err := storage.CreateUser("fran@weave.works")
	require.NoError(t, err)

	w := httptest.NewRecorder()
	r := requestAs(t, user, "POST", "/api/users/org/"+org.ExternalID+"/users", jsonBody{"email": fran.Email}.Reader(t))

	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)

	fran, err = storage.FindUserByEmail(fran.Email)
	require.NoError(t, err)
	require.Len(t, fran.Organizations, 1)
	assert.Equal(t, org.ID, fran.Organizations[0].ID)

	if assert.Len(t, sentEmails, 1) {
		assert.Equal(t, []string{fran.Email}, sentEmails[0].To)
		assert.Contains(t, string(sentEmails[0].Text), "has granted you access")
		assert.Contains(t, string(sentEmails[0].HTML), "has granted you access")
	}
}

func Test_Invite_UserInDifferentOrganization(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, org := getOrg(t)
	fran, _ := getOrg(t)

	w := httptest.NewRecorder()
	r := requestAs(t, user, "POST", "/api/users/org/"+org.ExternalID+"/users", jsonBody{"email": fran.Email}.Reader(t))

	app.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)

	fran, err := storage.FindUserByEmail(fran.Email)
	require.NoError(t, err)
	require.Len(t, fran.Organizations, 2)
	orgIDs := []string{fran.Organizations[0].ID, fran.Organizations[1].ID}
	assert.Contains(t, orgIDs, org.ID)

	if assert.Len(t, sentEmails, 1) {
		assert.Equal(t, []string{fran.Email}, sentEmails[0].To)
		assert.Contains(t, string(sentEmails[0].Text), "has granted you access")
		assert.Contains(t, string(sentEmails[0].HTML), "has granted you access")
	}
}

func Test_Invite_RemoveOtherUsersAccess(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, org := getOrg(t)
	otherUser := getApprovedUser(t)
	otherUser, _, err := storage.InviteUser(otherUser.Email, org.ExternalID)
	require.NoError(t, err)
	require.Len(t, otherUser.Organizations, 1)

	{
		w := httptest.NewRecorder()
		r := requestAs(t, user, "DELETE", "/api/users/org/"+org.ExternalID+"/users/"+otherUser.Email, nil)

		app.ServeHTTP(w, r)
		assert.Equal(t, http.StatusNoContent, w.Code)
	}

	otherUser, err = storage.FindUserByID(otherUser.ID)
	require.NoError(t, err)
	require.Len(t, otherUser.Organizations, 0)

	{
		w := httptest.NewRecorder()
		r := requestAs(t, user, "GET", "/api/users/org/"+org.ExternalID+"/users", nil)

		app.ServeHTTP(w, r)
		assert.Equal(t, http.StatusOK, w.Code)
		body := map[string]interface{}{}
		assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		assert.Equal(t, map[string]interface{}{
			"users": []interface{}{map[string]interface{}{"email": user.Email, "self": true}},
		}, body)
	}
}

func Test_Invite_RemoveMyOwnAccess(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, org := getOrg(t)

	w := httptest.NewRecorder()
	r := requestAs(t, user, "DELETE", "/api/users/org/"+org.ExternalID+"/users/"+user.Email, nil)

	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusNoContent, w.Code)

	user, err := storage.FindUserByID(user.ID)
	require.NoError(t, err)
	require.Len(t, user.Organizations, 0)
}

func Test_Invite_RemoveAccess_Forbidden(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, _ := getOrg(t)
	otherUser, otherOrg := getOrg(t)

	w := httptest.NewRecorder()
	r := requestAs(t, user, "DELETE", "/api/users/org/"+otherOrg.ExternalID+"/users/"+otherUser.Email, nil)

	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusForbidden, w.Code)

	otherUser, err := storage.FindUserByID(otherUser.ID)
	require.NoError(t, err)
	require.Len(t, otherUser.Organizations, 1)
}

func Test_Invite_RemoveAccess_NotFound(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, _ := getOrg(t)

	w := httptest.NewRecorder()
	r := requestAs(t, user, "DELETE", "/api/users/org/foobar/users/"+user.Email, nil)

	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusNotFound, w.Code)
}
