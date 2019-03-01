package api_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/api"
	"github.com/weaveworks/service/users/db/dbtest"
)

func doRequest(t *testing.T, user *users.User, method string, path string, body io.Reader, expectedStatus int) []byte {
	w := httptest.NewRecorder()

	r := requestAs(t, user, method, path, body)
	app.ServeHTTP(w, r)
	assert.Equal(t, expectedStatus, w.Code)
	if w.Body.Len() == 0 {
		return nil
	}
	return w.Body.Bytes()
}

func Test_RolesIsInTeamResponse(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, _, team := dbtest.GetOrgAndTeam(t, database)
	bodyBytes := doRequest(t, user, "GET", "/api/users/teams", nil, http.StatusOK)
	body := map[string]interface{}{}
	assert.NoError(t, json.Unmarshal(bodyBytes, &body))
	assert.Equal(t, map[string]interface{}{
		"teams": []interface{}{
			map[string]interface{}{
				"id":     team.ExternalID,
				"name":   team.Name,
				"roleId": "admin",
			},
		},
	}, body)
}

func Test_PermissionsInRoleResponse(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, _, _ := dbtest.GetOrgAndTeam(t, database)
	bodyBytes := doRequest(t, user, "GET", "/api/users/roles", nil, http.StatusOK)
	body := &api.RolesView{}
	assert.NoError(t, json.Unmarshal(bodyBytes, &body))

	// admin, editor, viewer
	assert.Equal(t, 3, len(body.Roles))

	var admin api.RoleView
	for _, role := range body.Roles {
		if role.ID == "admin" {
			admin = role
		}
	}

	assert.NotNil(t, admin)

	// should be some permissions!
	assert.NotZero(t, len(admin.Permissions))
}
