package api_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/db/dbtest"
)

func teamRequest(t *testing.T, user *users.User, method string, team string, additionalPath string, body io.Reader, expectedStatus int) map[string]interface{} {
	w := httptest.NewRecorder()
	path := "/api/users/teams/" + team
	if additionalPath != "" {
		path = path + additionalPath
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

func getTeamWithMembers(t *testing.T, count int) (team *users.Team, us []*users.User) {
	team = getTeam(t)

	for ; count > 0; count-- {
		u := getUser(t)
		us = append(us, u)

		database.AddUserToTeam(context.TODO(), u.ID, team.ID, users.AdminRoleID)
	}

	return team, us
}

func getMemberWithRole(membersWithRoles []*users.UserWithRole, userID string) *users.UserWithRole {
	var userWithRole *users.UserWithRole

	for _, u := range membersWithRoles {
		if u.User.ID == userID {
			userWithRole = u
		}
	}

	return userWithRole
}

func TestAPI_deleteTeam(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, org, _ := dbtest.GetOrgAndTeam(t, database)

	teams, err := database.ListTeamsForUserID(context.TODO(), user.ID)
	assert.NoError(t, err)
	assert.Len(t, teams, 1)

	{ // non-empty team
		teamRequest(t, user, "DELETE", org.TeamExternalID, "", nil, http.StatusForbidden)
	}
	// delete org
	err = database.DeleteOrganization(context.TODO(), org.ExternalID, user.ID)
	assert.NoError(t, err)
	{ // now empty team
		teamRequest(t, user, "DELETE", org.TeamExternalID, "", nil, http.StatusNoContent)
	}

	teams, err = database.ListTeamsForUserID(context.TODO(), user.ID)
	assert.NoError(t, err)
	assert.Len(t, teams, 0)
}

func TestAPI_RemoveUserFromTeam(t *testing.T) {
	setup(t)
	defer cleanup(t)

	team, members := getTeamWithMembers(t, 2)
	user := members[0]
	bUser := members[1]

	err := database.AddUserToTeam(context.TODO(), user.ID, team.ID, users.AdminRoleID)
	assert.NoError(t, err)
	err = database.AddUserToTeam(context.TODO(), bUser.ID, team.ID, users.EditorRoleID)
	assert.NoError(t, err)

	teams, _ := database.ListTeamsForUserID(context.TODO(), bUser.ID)
	assert.Len(t, teams, 1)

	teamRequest(t, user, "DELETE", team.ExternalID, "/users/"+bUser.Email, nil, http.StatusOK)

	teams, _ = database.ListTeamsForUserID(context.TODO(), bUser.ID)
	assert.Len(t, teams, 0)
}

func TestAPI_InviteUserToTeam(t *testing.T) {
	setup(t)
	defer cleanup(t)

	team, members := getTeamWithMembers(t, 1)
	user := members[0]
	bUser := dbtest.GetUser(t, database)

	teams, _ := database.ListTeamsForUserID(context.TODO(), bUser.ID)
	assert.Len(t, teams, 0)

	teamRequest(t, user, "POST", team.ExternalID, "/users", jsonBody{"email": bUser.Email}.Reader(t), http.StatusOK)

	teams, _ = database.ListTeamsForUserID(context.TODO(), bUser.ID)
	assert.Len(t, teams, 1)
}

func TestAPI_changeRole(t *testing.T) {
	setup(t)
	defer cleanup(t)

	team, members := getTeamWithMembers(t, 2)
	user := members[0]
	userB := members[1]

	assert.Len(t, sentEmails, 0)

	teamRequest(t, user, "PUT", team.ExternalID, "/users/"+userB.Email, jsonBody{"roleId": users.EditorRoleID}.Reader(t), http.StatusNoContent)

	membersWithRoles, err := database.ListTeamUsersWithRoles(context.TODO(), team.ID)
	assert.NoError(t, err)

	userBWithRole := getMemberWithRole(membersWithRoles, userB.ID)

	assert.Equal(t, userBWithRole.Role.ID, users.EditorRoleID)
}

func TestAPI_changeOwnRole(t *testing.T) {
	setup(t)
	defer cleanup(t)

	team, members := getTeamWithMembers(t, 1)
	user := members[0]

	membersWithRoles, _ := database.ListTeamUsersWithRoles(context.TODO(), team.ID)
	userWithRole := getMemberWithRole(membersWithRoles, user.ID)

	assert.Equal(t, userWithRole.Role.ID, users.AdminRoleID)

	teamRequest(t, user, "PUT", team.ExternalID, "/users/"+user.Email, jsonBody{"roleId": users.EditorRoleID}.Reader(t), http.StatusForbidden)

	membersWithRoles, _ = database.ListTeamUsersWithRoles(context.TODO(), team.ID)
	userWithRole = getMemberWithRole(membersWithRoles, user.ID)

	assert.Equal(t, userWithRole.Role.ID, users.AdminRoleID)
}

func TestAPI_RemoveOtherUsersAccess(t *testing.T) {
	setup(t)
	defer cleanup(t)

	team, members := getTeamWithMembers(t, 2)
	userA := members[0]
	userB := members[1]

	teams, _ := database.ListTeamsForUserID(context.TODO(), userB.ID)
	assert.Equal(t, len(teams), 1)

	teamRequest(t, userA, "DELETE", team.ExternalID, "/users/"+userB.Email, nil, http.StatusOK)

	teams, _ = database.ListTeamsForUserID(context.TODO(), userB.ID)
	assert.Equal(t, len(teams), 0)
}

func TestAPI_RemoveMyOwnAccess(t *testing.T) {
	setup(t)
	defer cleanup(t)

	team, members := getTeamWithMembers(t, 2)
	user := members[0]

	teams, _ := database.ListTeamsForUserID(context.TODO(), user.ID)
	assert.Len(t, teams, 1)

	teamRequest(t, user, "DELETE", team.ExternalID, "/users/"+user.Email, nil, http.StatusOK)

	teams, _ = database.ListTeamsForUserID(context.TODO(), user.ID)
	assert.Len(t, teams, 0)
}

func TestAPI_RemoveLastUser(t *testing.T) {
	setup(t)
	defer cleanup(t)

	team, members := getTeamWithMembers(t, 1)
	user := members[0]

	teams, _ := database.ListTeamsForUserID(context.TODO(), user.ID)
	assert.Len(t, teams, 1)

	teamRequest(t, user, "DELETE", team.ExternalID, user.Email, nil, http.StatusForbidden)

	teams, _ = database.ListTeamsForUserID(context.TODO(), user.ID)
	assert.Len(t, teams, 1)
}

func TestAPI_RemoveAccess_Forbidden(t *testing.T) {
	setup(t)
	defer cleanup(t)

	_, membersA := getTeamWithMembers(t, 1)
	userA := membersA[0]
	teamB, membersB := getTeamWithMembers(t, 1)
	userB := membersB[0]

	teams, _ := database.ListTeamsForUserID(context.TODO(), userB.ID)
	assert.Len(t, teams, 1)

	teamRequest(t, userA, "DELETE", teamB.ExternalID, userB.Email, nil, http.StatusForbidden)

	teams, _ = database.ListTeamsForUserID(context.TODO(), userB.ID)
	assert.Len(t, teams, 1)
}
