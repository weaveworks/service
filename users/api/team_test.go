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

func teamRequest(t *testing.T, user *users.User, method string, team string, email string, body io.Reader, expectedStatus int) map[string]interface{} {
	w := httptest.NewRecorder()
	path := "/api/users/teams/" + team
	if email != "" {
		path = path + "/users/" + email
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

func TestAPI_changeRole(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, us, org := getOrgWithMembers(t, 2)
	otherUser := us[0]

	assert.Len(t, sentEmails, 0)

	teamRequest(t, user, "PUT", org.TeamExternalID, otherUser.Email, jsonBody{"roleId": "editor"}.Reader(t), http.StatusNoContent)

	body := requestOrgAs(t, user, "GET", org.ExternalID, "", nil, http.StatusOK)
	assert.Equal(t, map[string]interface{}{
		"users": []interface{}{
			map[string]interface{}{
				"email":  otherUser.Email,
				"roleId": "editor",
			},
			map[string]interface{}{
				"email":  user.Email,
				"self":   true,
				"roleId": "admin",
			},
		},
	}, body)
}

func TestAPI_changeOwnRole(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, org, _ := dbtest.GetOrgAndTeam(t, database)

	teamRequest(t, user, "PUT", org.TeamExternalID, user.Email, jsonBody{"roleId": "editor"}.Reader(t), http.StatusForbidden)

	body := requestOrgAs(t, user, "GET", org.ExternalID, "", nil, http.StatusOK)
	assert.Equal(t, map[string]interface{}{
		"users": []interface{}{
			map[string]interface{}{
				"email":  user.Email,
				"self":   true,
				"roleId": "admin",
			},
		},
	}, body)
}
