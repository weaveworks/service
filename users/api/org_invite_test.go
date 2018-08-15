package api_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weaveworks/service/users"
)

// requestOrgAs sends a request for an organization as the given user,
// and asserts that the response has the given status. It returns the
// response body parsed as a JSON object.
func requestOrgAs(t *testing.T, user *users.User, method string, org string, email string, body io.Reader, expectedStatus int) map[string]interface{} {
	w := httptest.NewRecorder()
	path := "/api/users/org/" + org + "/users"
	if email != "" {
		path = path + "/" + email
	}
	r := requestAs(t, user, method, path, body)
	app.ServeHTTP(w, r)
	assert.Equal(t, expectedStatus, w.Code)
	if w.Body.Len() == 0 {
		return nil
	}
	responseBody := map[string]interface{}{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &responseBody))
	return responseBody
}

func requestInvite(t *testing.T, user *users.User, org *users.Organization, email string, expectedStatus int) map[string]interface{} {
	return requestOrgAs(t, user, "POST", org.ExternalID, "", jsonBody{"email": email}.Reader(t), expectedStatus)
}

// getOrgWithMembers populates the organization with given member count.
func getOrgWithMembers(t *testing.T, count int) (*users.User, []*users.User, *users.Organization) {
	owner, org := getOrg(t)
	members := []*users.User{}
	for ; count > 1; count-- {
		u := getUser(t)
		u, _, err := database.InviteUser(context.Background(), u.Email, org.ExternalID)
		require.NoError(t, err)
		members = append(members, u)
	}
	return owner, members, org
}

func assertEmailSent(t *testing.T, to string, contains string) {
	if assert.Len(t, sentEmails, 1) {
		assert.Equal(t, []string{to}, sentEmails[0].To)
		assert.Contains(t, string(sentEmails[0].Text), contains)
		assert.Contains(t, string(sentEmails[0].HTML), contains)
	}
}

func Test_InviteNonExistentUser(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, org := getOrg(t)
	franEmail := "fran@weave.works"
	body := requestInvite(t, user, org, franEmail, http.StatusOK)
	assert.Equal(t, map[string]interface{}{"email": franEmail}, body)

	fran, err := database.FindUserByEmail(context.Background(), franEmail)
	require.NoError(t, err)
	assert.Equal(t, fran.Email, franEmail)
	organizations, err := database.ListOrganizationsForUserIDs(context.Background(), fran.ID)
	require.NoError(t, err)
	require.Len(t, organizations, 1)
	assert.Equal(t, org.ID, organizations[0].ID)

	assertEmailSent(t, franEmail, "has invited you")
}

func Test_InviteExistingUser(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, org := getOrg(t)
	fran := getUser(t)

	body := requestInvite(t, user, org, fran.Email, http.StatusOK)
	assert.Equal(t, map[string]interface{}{"email": fran.Email}, body)

	organizations, err := database.ListOrganizationsForUserIDs(context.Background(), fran.ID)
	require.NoError(t, err)
	require.Len(t, organizations, 1)
	assert.Equal(t, org.ID, organizations[0].ID)

	assertEmailSent(t, fran.Email, "has granted you access")
}

func Test_Invite_WithInvalidJSON(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, org := getOrg(t)

	requestOrgAs(t, user, "POST", org.ExternalID, "", strings.NewReader(`garbage`), http.StatusBadRequest)

	assert.Len(t, sentEmails, 0)
}

func Test_Invite_WithBlankEmail(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, org := getOrg(t)

	body := requestInvite(t, user, org, "", http.StatusBadRequest)
	assert.Equal(t, map[string]interface{}{
		"errors": []interface{}{
			map[string]interface{}{
				"message": "email is not valid"},
		},
	}, body)

	assert.Len(t, sentEmails, 0)
}

func Test_Invite_UserAlreadyInSameOrganization(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, org := getOrg(t)

	fran, err := database.CreateUser(context.Background(), "fran@weave.works")
	require.NoError(t, err)
	fran, created, err := database.InviteUser(context.Background(), fran.Email, org.ExternalID)
	require.NoError(t, err)
	organizations, err := database.ListOrganizationsForUserIDs(context.Background(), fran.ID)
	require.NoError(t, err)
	require.Len(t, organizations, 1)
	assert.Equal(t, org.ID, organizations[0].ID)
	assert.Equal(t, created, false)

	requestInvite(t, user, org, fran.Email, http.StatusOK)

	organizations, err = database.ListOrganizationsForUserIDs(context.Background(), fran.ID)
	require.NoError(t, err)
	require.Len(t, organizations, 1)
	assert.Equal(t, org.ID, organizations[0].ID)

	assertEmailSent(t, fran.Email, "has granted you access")
}

func Test_Invite_UserToAnOrgIDontOwn(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user := getUser(t)
	otherUser := getUser(t)
	_, otherOrg := getOrg(t)

	requestInvite(t, user, otherOrg, otherUser.Email, http.StatusForbidden)

	organizations, err := database.ListOrganizationsForUserIDs(context.Background(), user.ID)
	require.NoError(t, err)
	require.Len(t, organizations, 0)
	assert.Len(t, sentEmails, 0)
}

func Test_Invite_UserInDifferentOrganization(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, org := getOrg(t)
	fran, _ := getOrg(t)

	requestInvite(t, user, org, fran.Email, http.StatusOK)

	organizations, err := database.ListOrganizationsForUserIDs(context.Background(), fran.ID)
	require.NoError(t, err)
	require.Len(t, organizations, 2)
	orgIDs := []string{organizations[0].ID, organizations[1].ID}
	assert.Contains(t, orgIDs, org.ID)

	assertEmailSent(t, fran.Email, "has granted you access")
}

func Test_Invite_RemoveOtherUsersAccess(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, us, org := getOrgWithMembers(t, 2)
	otherUser := us[0]
	organizations, err := database.ListOrganizationsForUserIDs(context.Background(), otherUser.ID)
	require.NoError(t, err)
	require.Len(t, organizations, 1)

	requestOrgAs(t, user, "DELETE", org.ExternalID, otherUser.Email, nil, http.StatusNoContent)

	organizations, err = database.ListOrganizationsForUserIDs(context.Background(), otherUser.ID)
	require.NoError(t, err)
	require.Len(t, organizations, 0)

	body := requestOrgAs(t, user, "GET", org.ExternalID, "", nil, http.StatusOK)
	assert.Equal(t, map[string]interface{}{
		"users": []interface{}{
			map[string]interface{}{
				"email": user.Email,
				"self":  true,
			},
		},
	}, body)
}

func Test_Invite_RemoveMyOwnAccess(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, _, org := getOrgWithMembers(t, 2)

	requestOrgAs(t, user, "DELETE", org.ExternalID, user.Email, nil, http.StatusNoContent)

	organizations, err := database.ListOrganizationsForUserIDs(context.Background(), user.ID)
	require.NoError(t, err)
	require.Len(t, organizations, 0)
}

func Test_Invite_RemoveLastUser(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, org := getOrg(t)

	requestOrgAs(t, user, "DELETE", org.ExternalID, user.Email, nil, http.StatusForbidden)

	organizations, err := database.ListOrganizationsForUserIDs(context.Background(), user.ID)
	require.NoError(t, err)
	require.Len(t, organizations, 1)
}

func Test_Invite_RemoveAccess_Forbidden(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, _, _ := getOrgWithMembers(t, 2)
	otherUser, _, otherOrg := getOrgWithMembers(t, 2)

	requestOrgAs(t, user, "DELETE", otherOrg.ExternalID, otherUser.Email, nil, http.StatusForbidden)

	organizations, err := database.ListOrganizationsForUserIDs(context.Background(), otherUser.ID)
	require.NoError(t, err)
	require.Len(t, organizations, 1)
}

func Test_Invite_RemoveAccess_NotFound(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, _ := getOrg(t)

	requestOrgAs(t, user, "DELETE", "foobar", user.Email, nil, http.StatusNotFound)
}
