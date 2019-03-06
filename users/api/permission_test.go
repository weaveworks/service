package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	billing_grpc "github.com/weaveworks/service/common/billing/grpc"
	"github.com/weaveworks/service/common/billing/provider"
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

func Test_PermissionInviteTeamMember(t *testing.T) {
	setup(t)
	defer cleanup(t)

	_, org, team := dbtest.GetOrgAndTeam(t, database)
	database.SetFeatureFlags(context.TODO(), org.ExternalID, []string{"permissions"})

	path := fmt.Sprintf("/api/users/teams/%s/users", team.ExternalID)
	requestBody, _ := json.Marshal(map[string]string{
		"email":  "blu@blu.blu",
		"roleId": "viewer",
	})

	viewer, _ := dbtest.GetUserInTeam(t, database, team, "viewer")
	editor, _ := dbtest.GetUserInTeam(t, database, team, "editor")
	admin, _ := dbtest.GetUserInTeam(t, database, team, "admin")
	doRequest(t, viewer, "POST", path, bytes.NewReader(requestBody), http.StatusForbidden)
	doRequest(t, editor, "POST", path, bytes.NewReader(requestBody), http.StatusForbidden)
	doRequest(t, admin, "POST", path, bytes.NewReader(requestBody), http.StatusOK)
}
func Test_PermissionUpdateTeamMemberRole(t *testing.T) {
	setup(t)
	defer cleanup(t)

	_, org, team := dbtest.GetOrgAndTeam(t, database)
	database.SetFeatureFlags(context.TODO(), org.ExternalID, []string{"permissions"})

	other, _ := dbtest.GetUserInTeam(t, database, team, "viewer")
	path := fmt.Sprintf("/api/users/teams/%s/users/%s", team.ExternalID, other.Email)
	requestBody, _ := json.Marshal(map[string]string{
		"roleId": "viewer",
	})

	viewer, _ := dbtest.GetUserInTeam(t, database, team, "viewer")
	editor, _ := dbtest.GetUserInTeam(t, database, team, "editor")
	admin, _ := dbtest.GetUserInTeam(t, database, team, "admin")
	doRequest(t, viewer, "PUT", path, bytes.NewReader(requestBody), http.StatusForbidden)
	doRequest(t, editor, "PUT", path, bytes.NewReader(requestBody), http.StatusForbidden)
	doRequest(t, admin, "PUT", path, bytes.NewReader(requestBody), http.StatusNoContent)
}
func Test_PermissionRemoveTeamMember(t *testing.T) {
	setup(t)
	defer cleanup(t)

	_, org, team := dbtest.GetOrgAndTeam(t, database)
	database.SetFeatureFlags(context.TODO(), org.ExternalID, []string{"permissions"})

	other, _ := dbtest.GetUserInTeam(t, database, team, "viewer")
	path := fmt.Sprintf("/api/users/teams/%s/users/%s", team.ExternalID, other.Email)

	viewer, _ := dbtest.GetUserInTeam(t, database, team, "viewer")
	editor, _ := dbtest.GetUserInTeam(t, database, team, "editor")
	admin, _ := dbtest.GetUserInTeam(t, database, team, "admin")
	doRequest(t, viewer, "DELETE", path, nil, http.StatusForbidden)
	doRequest(t, editor, "DELETE", path, nil, http.StatusForbidden)
	doRequest(t, admin, "DELETE", path, nil, http.StatusOK)
}
func Test_PermissionViewTeamMembers(t *testing.T) {
	setup(t)
	defer cleanup(t)

	_, org, team := dbtest.GetOrgAndTeam(t, database)
	database.SetFeatureFlags(context.TODO(), org.ExternalID, []string{"permissions"})

	path := fmt.Sprintf("/api/users/org/%s/users", org.ExternalID)

	viewer, _ := dbtest.GetUserInTeam(t, database, team, "viewer")
	editor, _ := dbtest.GetUserInTeam(t, database, team, "editor")
	admin, _ := dbtest.GetUserInTeam(t, database, team, "admin")
	doRequest(t, viewer, "GET", path, nil, http.StatusOK)
	doRequest(t, editor, "GET", path, nil, http.StatusOK)
	doRequest(t, admin, "GET", path, nil, http.StatusOK)
}
func Test_PermissionDeleteInstance(t *testing.T) {
	setup(t)
	defer cleanup(t)

	_, org, team := dbtest.GetOrgAndTeam(t, database)
	database.SetFeatureFlags(context.TODO(), org.ExternalID, []string{"permissions"})

	path := fmt.Sprintf("/api/users/org/%s", org.ExternalID)

	viewer, _ := dbtest.GetUserInTeam(t, database, team, "viewer")
	editor, _ := dbtest.GetUserInTeam(t, database, team, "editor")
	admin, _ := dbtest.GetUserInTeam(t, database, team, "admin")
	doRequest(t, viewer, "DELETE", path, nil, http.StatusForbidden)
	doRequest(t, editor, "DELETE", path, nil, http.StatusForbidden)
	doRequest(t, admin, "DELETE", path, nil, http.StatusNoContent)
}

func Test_PermissionViewToken(t *testing.T) {
	setup(t)
	defer cleanup(t)

	_, org, team := dbtest.GetOrgAndTeam(t, database)
	database.SetFeatureFlags(context.TODO(), org.ExternalID, []string{"permissions"})

	body := map[string]interface{}{}
	path := fmt.Sprintf("/api/users/org/%s", org.ExternalID)

	viewer, _ := dbtest.GetUserInTeam(t, database, team, "viewer")
	editor, _ := dbtest.GetUserInTeam(t, database, team, "editor")
	admin, _ := dbtest.GetUserInTeam(t, database, team, "admin")
	bodyBytes := doRequest(t, viewer, "GET", path, nil, http.StatusOK)
	assert.NoError(t, json.Unmarshal(bodyBytes, &body))
	assert.Equal(t, nil, body["probeToken"])
	bodyBytes = doRequest(t, editor, "GET", path, nil, http.StatusOK)
	assert.NoError(t, json.Unmarshal(bodyBytes, &body))
	assert.Equal(t, nil, body["probeToken"])
	bodyBytes = doRequest(t, admin, "GET", path, nil, http.StatusOK)
	assert.NoError(t, json.Unmarshal(bodyBytes, &body))
	assert.NotEqual(t, nil, body["probeToken"])
}

func Test_PermissionTransferInstance(t *testing.T) {
	setup(t)
	defer cleanup(t)

	_, org, team := dbtest.GetOrgAndTeam(t, database)
	database.SetFeatureFlags(context.TODO(), org.ExternalID, []string{"permissions"})
	_, otherOrg, otherTeam := dbtest.GetOrgAndTeam(t, database)
	database.SetFeatureFlags(context.TODO(), otherOrg.ExternalID, []string{"permissions"})

	path := fmt.Sprintf("/api/users/org/%s", org.ExternalID)
	requestBody, _ := json.Marshal(map[string]string{
		"teamId": otherTeam.ExternalID,
	})

	billingClient.EXPECT().
		FindBillingAccountByTeamID(gomock.Any(), gomock.Any()).AnyTimes().
		Return(&billing_grpc.BillingAccount{
			ID:        1,
			CreatedAt: time.Now(),
			Provider:  provider.External,
		}, nil)

	viewer, _ := dbtest.GetUserInTeam(t, database, team, "viewer")
	editor, _ := dbtest.GetUserInTeam(t, database, team, "editor")
	admin, _ := dbtest.GetUserInTeam(t, database, team, "admin")
	// Viewers in the other team
	database.AddUserToTeam(context.TODO(), viewer.ID, otherTeam.ID, "viewer")
	database.AddUserToTeam(context.TODO(), editor.ID, otherTeam.ID, "viewer")
	database.AddUserToTeam(context.TODO(), admin.ID, otherTeam.ID, "viewer")
	doRequest(t, viewer, "PUT", path, bytes.NewReader(requestBody), http.StatusForbidden)
	doRequest(t, editor, "PUT", path, bytes.NewReader(requestBody), http.StatusForbidden)
	doRequest(t, admin, "PUT", path, bytes.NewReader(requestBody), http.StatusForbidden)
	// Editors in the other team
	database.UpdateUserRoleInTeam(context.TODO(), viewer.ID, otherTeam.ID, "editor")
	database.UpdateUserRoleInTeam(context.TODO(), editor.ID, otherTeam.ID, "editor")
	database.UpdateUserRoleInTeam(context.TODO(), admin.ID, otherTeam.ID, "editor")
	doRequest(t, viewer, "PUT", path, bytes.NewReader(requestBody), http.StatusForbidden)
	doRequest(t, editor, "PUT", path, bytes.NewReader(requestBody), http.StatusForbidden)
	doRequest(t, admin, "PUT", path, bytes.NewReader(requestBody), http.StatusForbidden)
	// Admins in the other team
	database.UpdateUserRoleInTeam(context.TODO(), viewer.ID, otherTeam.ID, "admin")
	database.UpdateUserRoleInTeam(context.TODO(), editor.ID, otherTeam.ID, "admin")
	database.UpdateUserRoleInTeam(context.TODO(), admin.ID, otherTeam.ID, "admin")
	doRequest(t, viewer, "PUT", path, bytes.NewReader(requestBody), http.StatusForbidden)
	doRequest(t, editor, "PUT", path, bytes.NewReader(requestBody), http.StatusForbidden)
	doRequest(t, admin, "PUT", path, bytes.NewReader(requestBody), http.StatusNoContent)
}
