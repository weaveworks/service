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
	"github.com/weaveworks/service/common/permission"
	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/api"
	"github.com/weaveworks/service/users/client"
	"github.com/weaveworks/service/users/db/dbtest"
	"github.com/weaveworks/service/users/mock_users"
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

func usersClientMock(org *users.Organization, u []*users.User, permissionID string) *mock_users.MockUsersClient {
	c := mock_users.NewMockUsersClient(ctrl)
	for _, user := range u {
		c.EXPECT().
			RequireOrgMemberPermissionTo(gomock.Any(), &users.RequireOrgMemberPermissionToRequest{
				OrgID:        &users.RequireOrgMemberPermissionToRequest_OrgExternalID{},
				UserID:       user.ID,
				PermissionID: permissionID,
			}).
			Return(nil, api.RequireOrgMemberPermissionTo(context.Background(), database, user.ID, org.ExternalID, permissionID))
	}
	return c
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
				"roleId": users.AdminRoleID,
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
		if role.ID == users.AdminRoleID {
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

	_, _, team := dbtest.GetOrgAndTeam(t, database)

	path := fmt.Sprintf("/api/users/teams/%s/users", team.ExternalID)
	requestBody, _ := json.Marshal(map[string]string{
		"email":  "blu@blu.blu",
		"roleId": users.ViewerRoleID,
	})

	viewer, _ := dbtest.GetUserInTeam(t, database, team, users.ViewerRoleID)
	editor, _ := dbtest.GetUserInTeam(t, database, team, users.EditorRoleID)
	admin, _ := dbtest.GetUserInTeam(t, database, team, users.AdminRoleID)
	doRequest(t, viewer, "POST", path, bytes.NewReader(requestBody), http.StatusForbidden)
	doRequest(t, editor, "POST", path, bytes.NewReader(requestBody), http.StatusForbidden)
	doRequest(t, admin, "POST", path, bytes.NewReader(requestBody), http.StatusOK)
}
func Test_PermissionUpdateTeamMemberRole(t *testing.T) {
	setup(t)
	defer cleanup(t)

	_, _, team := dbtest.GetOrgAndTeam(t, database)

	other, _ := dbtest.GetUserInTeam(t, database, team, users.ViewerRoleID)
	path := fmt.Sprintf("/api/users/teams/%s/users/%s", team.ExternalID, other.Email)
	requestBody, _ := json.Marshal(map[string]string{
		"roleId": users.ViewerRoleID,
	})

	viewer, _ := dbtest.GetUserInTeam(t, database, team, users.ViewerRoleID)
	editor, _ := dbtest.GetUserInTeam(t, database, team, users.EditorRoleID)
	admin, _ := dbtest.GetUserInTeam(t, database, team, users.AdminRoleID)
	doRequest(t, viewer, "PUT", path, bytes.NewReader(requestBody), http.StatusForbidden)
	doRequest(t, editor, "PUT", path, bytes.NewReader(requestBody), http.StatusForbidden)
	doRequest(t, admin, "PUT", path, bytes.NewReader(requestBody), http.StatusNoContent)
}
func Test_PermissionRemoveTeamMember(t *testing.T) {
	setup(t)
	defer cleanup(t)

	_, _, team := dbtest.GetOrgAndTeam(t, database)

	other, _ := dbtest.GetUserInTeam(t, database, team, users.ViewerRoleID)
	path := fmt.Sprintf("/api/users/teams/%s/users/%s", team.ExternalID, other.Email)

	viewer, _ := dbtest.GetUserInTeam(t, database, team, users.ViewerRoleID)
	editor, _ := dbtest.GetUserInTeam(t, database, team, users.EditorRoleID)
	admin, _ := dbtest.GetUserInTeam(t, database, team, users.AdminRoleID)
	doRequest(t, viewer, "DELETE", path, nil, http.StatusForbidden)
	doRequest(t, editor, "DELETE", path, nil, http.StatusForbidden)
	doRequest(t, admin, "DELETE", path, nil, http.StatusOK)
}
func Test_PermissionViewTeamMembers(t *testing.T) {
	setup(t)
	defer cleanup(t)

	_, org, team := dbtest.GetOrgAndTeam(t, database)

	path := fmt.Sprintf("/api/users/org/%s/users", org.ExternalID)

	viewer, _ := dbtest.GetUserInTeam(t, database, team, users.ViewerRoleID)
	editor, _ := dbtest.GetUserInTeam(t, database, team, users.EditorRoleID)
	admin, _ := dbtest.GetUserInTeam(t, database, team, users.AdminRoleID)
	doRequest(t, viewer, "GET", path, nil, http.StatusOK)
	doRequest(t, editor, "GET", path, nil, http.StatusOK)
	doRequest(t, admin, "GET", path, nil, http.StatusOK)
}
func Test_PermissionDeleteInstance(t *testing.T) {
	setup(t)
	defer cleanup(t)

	_, org, team := dbtest.GetOrgAndTeam(t, database)

	path := fmt.Sprintf("/api/users/org/%s", org.ExternalID)

	viewer, _ := dbtest.GetUserInTeam(t, database, team, users.ViewerRoleID)
	editor, _ := dbtest.GetUserInTeam(t, database, team, users.EditorRoleID)
	admin, _ := dbtest.GetUserInTeam(t, database, team, users.AdminRoleID)
	doRequest(t, viewer, "DELETE", path, nil, http.StatusForbidden)
	doRequest(t, editor, "DELETE", path, nil, http.StatusForbidden)
	doRequest(t, admin, "DELETE", path, nil, http.StatusNoContent)
}

func Test_PermissionViewToken(t *testing.T) {
	setup(t)
	defer cleanup(t)

	_, org, team := dbtest.GetOrgAndTeam(t, database)

	body := map[string]interface{}{}
	path := fmt.Sprintf("/api/users/org/%s", org.ExternalID)

	viewer, _ := dbtest.GetUserInTeam(t, database, team, users.ViewerRoleID)
	editor, _ := dbtest.GetUserInTeam(t, database, team, users.EditorRoleID)
	admin, _ := dbtest.GetUserInTeam(t, database, team, users.AdminRoleID)
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
	_, _, otherTeam := dbtest.GetOrgAndTeam(t, database)

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

	viewer, _ := dbtest.GetUserInTeam(t, database, team, users.ViewerRoleID)
	editor, _ := dbtest.GetUserInTeam(t, database, team, users.EditorRoleID)
	admin, _ := dbtest.GetUserInTeam(t, database, team, users.AdminRoleID)
	// Viewers in the other team
	database.AddUserToTeam(context.TODO(), viewer.ID, otherTeam.ID, users.ViewerRoleID)
	database.AddUserToTeam(context.TODO(), editor.ID, otherTeam.ID, users.ViewerRoleID)
	database.AddUserToTeam(context.TODO(), admin.ID, otherTeam.ID, users.ViewerRoleID)
	doRequest(t, viewer, "PUT", path, bytes.NewReader(requestBody), http.StatusForbidden)
	doRequest(t, editor, "PUT", path, bytes.NewReader(requestBody), http.StatusForbidden)
	doRequest(t, admin, "PUT", path, bytes.NewReader(requestBody), http.StatusForbidden)
	// Editors in the other team
	database.UpdateUserRoleInTeam(context.TODO(), viewer.ID, otherTeam.ID, users.EditorRoleID)
	database.UpdateUserRoleInTeam(context.TODO(), editor.ID, otherTeam.ID, users.EditorRoleID)
	database.UpdateUserRoleInTeam(context.TODO(), admin.ID, otherTeam.ID, users.EditorRoleID)
	doRequest(t, viewer, "PUT", path, bytes.NewReader(requestBody), http.StatusForbidden)
	doRequest(t, editor, "PUT", path, bytes.NewReader(requestBody), http.StatusForbidden)
	doRequest(t, admin, "PUT", path, bytes.NewReader(requestBody), http.StatusForbidden)
	// Admins in the other team
	database.UpdateUserRoleInTeam(context.TODO(), viewer.ID, otherTeam.ID, users.AdminRoleID)
	database.UpdateUserRoleInTeam(context.TODO(), editor.ID, otherTeam.ID, users.AdminRoleID)
	database.UpdateUserRoleInTeam(context.TODO(), admin.ID, otherTeam.ID, users.AdminRoleID)
	doRequest(t, viewer, "PUT", path, bytes.NewReader(requestBody), http.StatusForbidden)
	doRequest(t, editor, "PUT", path, bytes.NewReader(requestBody), http.StatusForbidden)
	doRequest(t, admin, "PUT", path, bytes.NewReader(requestBody), http.StatusNoContent)
}

type mockHTTPHandler struct {
	r *http.Request
}

func (h *mockHTTPHandler) ServeHTTP(_ http.ResponseWriter, req *http.Request) {
	h.r = req
}

func testMiddleware(t *testing.T, middleware *client.UserPermissionsMiddleware, user *users.User, method string, path string, expectedStatus int) {
	w := httptest.NewRecorder()
	r := requestAs(t, user, method, path, nil)
	r.Header.Set(middleware.UserIDHeader, user.ID)

	middleware.Wrap(&mockHTTPHandler{}).ServeHTTP(w, r)
	assert.Equal(t, expectedStatus, w.Code)
}

func Test_PermissionUpdateAlertingSettings(t *testing.T) {
	setup(t)
	defer cleanup(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, org, team := dbtest.GetOrgAndTeam(t, database)
	viewer, _ := dbtest.GetUserInTeam(t, database, team, users.ViewerRoleID)
	editor, _ := dbtest.GetUserInTeam(t, database, team, users.EditorRoleID)
	admin, _ := dbtest.GetUserInTeam(t, database, team, users.AdminRoleID)

	middleware := client.UserPermissionsMiddleware{
		UsersClient:  usersClientMock(org, []*users.User{viewer, editor, admin}, permission.UpdateAlertingSettings),
		UserIDHeader: "UserID",
	}
	path := fmt.Sprintf("/api/app/%s/api/prom/configs/rules", org.ExternalID)

	testMiddleware(t, &middleware, viewer, "POST", path, http.StatusBadGateway)
	testMiddleware(t, &middleware, editor, "POST", path, http.StatusOK)
	testMiddleware(t, &middleware, admin, "POST", path, http.StatusOK)
}

func Test_PermissionOpenHostShell(t *testing.T) {
	setup(t)
	defer cleanup(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, org, team := dbtest.GetOrgAndTeam(t, database)
	viewer, _ := dbtest.GetUserInTeam(t, database, team, users.ViewerRoleID)
	editor, _ := dbtest.GetUserInTeam(t, database, team, users.EditorRoleID)
	admin, _ := dbtest.GetUserInTeam(t, database, team, users.AdminRoleID)

	middleware := client.UserPermissionsMiddleware{
		UsersClient:  usersClientMock(org, []*users.User{viewer, editor, admin}, permission.OpenHostShell),
		UserIDHeader: "UserID",
	}
	path := fmt.Sprintf("/api/app/%s/api/control/2ds3dcccesdkf7b/ip-43-43-120-4-host/host_exec", org.ExternalID)

	testMiddleware(t, &middleware, viewer, "POST", path, http.StatusBadGateway)
	testMiddleware(t, &middleware, editor, "POST", path, http.StatusBadGateway)
	testMiddleware(t, &middleware, admin, "POST", path, http.StatusOK)
}
func Test_PermissionOpenContainerShell(t *testing.T) {
	setup(t)
	defer cleanup(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, org, team := dbtest.GetOrgAndTeam(t, database)
	viewer, _ := dbtest.GetUserInTeam(t, database, team, users.ViewerRoleID)
	editor, _ := dbtest.GetUserInTeam(t, database, team, users.EditorRoleID)
	admin, _ := dbtest.GetUserInTeam(t, database, team, users.AdminRoleID)

	middleware := client.UserPermissionsMiddleware{
		UsersClient:  usersClientMock(org, []*users.User{viewer, editor, admin}, permission.OpenContainerShell),
		UserIDHeader: "UserID",
	}
	path := fmt.Sprintf("/api/app/%s/api/control/45ea4112d38b6bf5/d63bbd1efac7cc746867c8affkc05e701fa11d40fe28e2a4cf28bd7bd4cdb3a1/docker_exec_container", org.ExternalID)

	testMiddleware(t, &middleware, viewer, "POST", path, http.StatusBadGateway)
	testMiddleware(t, &middleware, editor, "POST", path, http.StatusBadGateway)
	testMiddleware(t, &middleware, admin, "POST", path, http.StatusOK)
}
func Test_PermissionAttachToContainer(t *testing.T) {
	setup(t)
	defer cleanup(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, org, team := dbtest.GetOrgAndTeam(t, database)
	viewer, _ := dbtest.GetUserInTeam(t, database, team, users.ViewerRoleID)
	editor, _ := dbtest.GetUserInTeam(t, database, team, users.EditorRoleID)
	admin, _ := dbtest.GetUserInTeam(t, database, team, users.AdminRoleID)

	middleware := client.UserPermissionsMiddleware{
		UsersClient:  usersClientMock(org, []*users.User{viewer, editor, admin}, permission.AttachToContainer),
		UserIDHeader: "UserID",
	}
	path := fmt.Sprintf("/api/app/%s/api/control/45ea4112d38b6bf5/d63bbd1efac7cc746867c8affkc05e701fa11d40fe28e2a4cf28bd7bd4cdb3a1/docker_attach_container", org.ExternalID)

	testMiddleware(t, &middleware, viewer, "POST", path, http.StatusBadGateway)
	testMiddleware(t, &middleware, editor, "POST", path, http.StatusOK)
	testMiddleware(t, &middleware, admin, "POST", path, http.StatusOK)
}
func Test_PermissionPauseContainer(t *testing.T) {
	setup(t)
	defer cleanup(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, org, team := dbtest.GetOrgAndTeam(t, database)
	viewer, _ := dbtest.GetUserInTeam(t, database, team, users.ViewerRoleID)
	editor, _ := dbtest.GetUserInTeam(t, database, team, users.EditorRoleID)
	admin, _ := dbtest.GetUserInTeam(t, database, team, users.AdminRoleID)

	middleware := client.UserPermissionsMiddleware{
		UsersClient:  usersClientMock(org, []*users.User{viewer, editor, admin}, permission.PauseContainer),
		UserIDHeader: "UserID",
	}
	path := fmt.Sprintf("/api/app/%s/api/control/45ea4112d38b6bf5/d63bbd1efac7cc746867c8affkc05e701fa11d40fe28e2a4cf28bd7bd4cdb3a1/docker_pause_container", org.ExternalID)

	testMiddleware(t, &middleware, viewer, "POST", path, http.StatusBadGateway)
	testMiddleware(t, &middleware, editor, "POST", path, http.StatusOK)
	testMiddleware(t, &middleware, admin, "POST", path, http.StatusOK)

	middleware.UsersClient = usersClientMock(org, []*users.User{viewer, editor, admin}, permission.PauseContainer)
	path = fmt.Sprintf("/api/app/%s/api/control/45ea4112d38b6bf5/d63bbd1efac7cc746867c8affkc05e701fa11d40fe28e2a4cf28bd7bd4cdb3a1/docker_unpause_container", org.ExternalID)

	testMiddleware(t, &middleware, viewer, "POST", path, http.StatusBadGateway)
	testMiddleware(t, &middleware, editor, "POST", path, http.StatusOK)
	testMiddleware(t, &middleware, admin, "POST", path, http.StatusOK)
}

func Test_PermissionRestartContainer(t *testing.T) {
	setup(t)
	defer cleanup(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, org, team := dbtest.GetOrgAndTeam(t, database)
	viewer, _ := dbtest.GetUserInTeam(t, database, team, users.ViewerRoleID)
	editor, _ := dbtest.GetUserInTeam(t, database, team, users.EditorRoleID)
	admin, _ := dbtest.GetUserInTeam(t, database, team, users.AdminRoleID)

	middleware := client.UserPermissionsMiddleware{
		UsersClient:  usersClientMock(org, []*users.User{viewer, editor, admin}, permission.RestartContainer),
		UserIDHeader: "UserID",
	}
	path := fmt.Sprintf("/api/app/%s/api/control/45ea4112d38b6bf5/d63bbd1efac7cc746867c8affkc05e701fa11d40fe28e2a4cf28bd7bd4cdb3a1/docker_restart_container", org.ExternalID)

	testMiddleware(t, &middleware, viewer, "POST", path, http.StatusBadGateway)
	testMiddleware(t, &middleware, editor, "POST", path, http.StatusOK)
	testMiddleware(t, &middleware, admin, "POST", path, http.StatusOK)
}

func Test_PermissionStopContainer(t *testing.T) {
	setup(t)
	defer cleanup(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, org, team := dbtest.GetOrgAndTeam(t, database)
	viewer, _ := dbtest.GetUserInTeam(t, database, team, users.ViewerRoleID)
	editor, _ := dbtest.GetUserInTeam(t, database, team, users.EditorRoleID)
	admin, _ := dbtest.GetUserInTeam(t, database, team, users.AdminRoleID)

	middleware := client.UserPermissionsMiddleware{
		UsersClient:  usersClientMock(org, []*users.User{viewer, editor, admin}, permission.StopContainer),
		UserIDHeader: "UserID",
	}
	path := fmt.Sprintf("/api/app/%s/api/control/45ea4112d38b6bf5/d63bbd1efac7cc746867c8affkc05e701fa11d40fe28e2a4cf28bd7bd4cdb3a1/docker_stop_container", org.ExternalID)

	testMiddleware(t, &middleware, viewer, "POST", path, http.StatusBadGateway)
	testMiddleware(t, &middleware, editor, "POST", path, http.StatusOK)
	testMiddleware(t, &middleware, admin, "POST", path, http.StatusOK)
}

func Test_PermissionViewPodLogs(t *testing.T) {
	setup(t)
	defer cleanup(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, org, team := dbtest.GetOrgAndTeam(t, database)
	viewer, _ := dbtest.GetUserInTeam(t, database, team, users.ViewerRoleID)
	editor, _ := dbtest.GetUserInTeam(t, database, team, users.EditorRoleID)
	admin, _ := dbtest.GetUserInTeam(t, database, team, users.AdminRoleID)

	middleware := client.UserPermissionsMiddleware{
		UsersClient:  usersClientMock(org, []*users.User{viewer, editor, admin}, permission.ViewPodLogs),
		UserIDHeader: "UserID",
	}
	path := fmt.Sprintf("/api/app/%s/api/control/45ea4112d38b6bf5/ebc1e081-43ec-11e9-ad34-34324/kubernetes_get_logs", org.ExternalID)

	testMiddleware(t, &middleware, viewer, "POST", path, http.StatusOK)
	testMiddleware(t, &middleware, editor, "POST", path, http.StatusOK)
	testMiddleware(t, &middleware, admin, "POST", path, http.StatusOK)
}

func Test_PermissionUpdateReplicaCount(t *testing.T) {
	setup(t)
	defer cleanup(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, org, team := dbtest.GetOrgAndTeam(t, database)
	viewer, _ := dbtest.GetUserInTeam(t, database, team, users.ViewerRoleID)
	editor, _ := dbtest.GetUserInTeam(t, database, team, users.EditorRoleID)
	admin, _ := dbtest.GetUserInTeam(t, database, team, users.AdminRoleID)

	middleware := client.UserPermissionsMiddleware{
		UsersClient:  usersClientMock(org, []*users.User{viewer, editor, admin}, permission.UpdateReplicaCount),
		UserIDHeader: "UserID",
	}
	path := fmt.Sprintf("/api/app/%s/api/control/45ea4112d38b6bf5/ebc1e081-43ec-11e9-ad34-34324/kubernetes_scale_up", org.ExternalID)

	testMiddleware(t, &middleware, viewer, "POST", path, http.StatusBadGateway)
	testMiddleware(t, &middleware, editor, "POST", path, http.StatusOK)
	testMiddleware(t, &middleware, admin, "POST", path, http.StatusOK)

	middleware.UsersClient = usersClientMock(org, []*users.User{viewer, editor, admin}, permission.UpdateReplicaCount)
	path = fmt.Sprintf("/api/app/%s/api/control/45ea4112d38b6bf5/ebc1e081-43ec-11e9-ad34-34324/kubernetes_scale_down", org.ExternalID)

	testMiddleware(t, &middleware, viewer, "POST", path, http.StatusBadGateway)
	testMiddleware(t, &middleware, editor, "POST", path, http.StatusOK)
	testMiddleware(t, &middleware, admin, "POST", path, http.StatusOK)
}

func Test_PermissionDeletePod(t *testing.T) {
	setup(t)
	defer cleanup(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, org, team := dbtest.GetOrgAndTeam(t, database)
	viewer, _ := dbtest.GetUserInTeam(t, database, team, users.ViewerRoleID)
	editor, _ := dbtest.GetUserInTeam(t, database, team, users.EditorRoleID)
	admin, _ := dbtest.GetUserInTeam(t, database, team, users.AdminRoleID)

	middleware := client.UserPermissionsMiddleware{
		UsersClient:  usersClientMock(org, []*users.User{viewer, editor, admin}, permission.DeletePod),
		UserIDHeader: "UserID",
	}
	path := fmt.Sprintf("/api/app/%s/api/control/45ea4112d38b6bf5/ebc1e081-43ec-11e9-ad34-34324/kubernetes_delete_pod", org.ExternalID)

	testMiddleware(t, &middleware, viewer, "POST", path, http.StatusBadGateway)
	testMiddleware(t, &middleware, editor, "POST", path, http.StatusOK)
	testMiddleware(t, &middleware, admin, "POST", path, http.StatusOK)
}

func Test_PermissionDeployImage(t *testing.T) {
	setup(t)
	defer cleanup(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, org, team := dbtest.GetOrgAndTeam(t, database)
	viewer, _ := dbtest.GetUserInTeam(t, database, team, users.ViewerRoleID)
	editor, _ := dbtest.GetUserInTeam(t, database, team, users.EditorRoleID)
	admin, _ := dbtest.GetUserInTeam(t, database, team, users.AdminRoleID)

	middleware := client.UserPermissionsMiddleware{
		UsersClient:  usersClientMock(org, []*users.User{viewer, editor, admin}, permission.DeployImage),
		UserIDHeader: "UserID",
	}
	path := fmt.Sprintf("/api/app/%s/api/flux/v9/update-manifests", org.ExternalID)

	testMiddleware(t, &middleware, viewer, "POST", path, http.StatusBadGateway)
	testMiddleware(t, &middleware, editor, "POST", path, http.StatusOK)
	testMiddleware(t, &middleware, admin, "POST", path, http.StatusOK)

	middleware.UsersClient = usersClientMock(org, []*users.User{viewer, editor, admin}, permission.DeployImage)
	path = fmt.Sprintf("/api/app/%s/api/flux/v6/update-images", org.ExternalID)

	testMiddleware(t, &middleware, viewer, "POST", path, http.StatusBadGateway)
	testMiddleware(t, &middleware, editor, "POST", path, http.StatusOK)
	testMiddleware(t, &middleware, admin, "POST", path, http.StatusOK)
}

func Test_PermissionUpdateDeploymentPolicy(t *testing.T) {
	setup(t)
	defer cleanup(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, org, team := dbtest.GetOrgAndTeam(t, database)
	viewer, _ := dbtest.GetUserInTeam(t, database, team, users.ViewerRoleID)
	editor, _ := dbtest.GetUserInTeam(t, database, team, users.EditorRoleID)
	admin, _ := dbtest.GetUserInTeam(t, database, team, users.AdminRoleID)

	middleware := client.UserPermissionsMiddleware{
		UsersClient:  usersClientMock(org, []*users.User{viewer, editor, admin}, permission.UpdateDeploymentPolicy),
		UserIDHeader: "UserID",
	}
	path := fmt.Sprintf("/api/app/%s/api/flux/v6/policies", org.ExternalID)

	testMiddleware(t, &middleware, viewer, "PATCH", path, http.StatusBadGateway)
	testMiddleware(t, &middleware, editor, "PATCH", path, http.StatusOK)
	testMiddleware(t, &middleware, admin, "PATCH", path, http.StatusOK)
}

func Test_PermissionUpdateNotificationSettings(t *testing.T) {
	setup(t)
	defer cleanup(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, org, team := dbtest.GetOrgAndTeam(t, database)
	viewer, _ := dbtest.GetUserInTeam(t, database, team, users.ViewerRoleID)
	editor, _ := dbtest.GetUserInTeam(t, database, team, users.EditorRoleID)
	admin, _ := dbtest.GetUserInTeam(t, database, team, users.AdminRoleID)

	middleware := client.UserPermissionsMiddleware{
		UsersClient:  usersClientMock(org, []*users.User{viewer, editor, admin}, permission.UpdateNotificationSettings),
		UserIDHeader: "UserID",
	}
	path := fmt.Sprintf("/api/app/%s/api/notification/config/receivers/97b52281-5c6d-4250-b9a7-ad8a65427fac", org.ExternalID)

	testMiddleware(t, &middleware, viewer, "POST", path, http.StatusBadGateway)
	testMiddleware(t, &middleware, editor, "POST", path, http.StatusBadGateway)
	testMiddleware(t, &middleware, admin, "POST", path, http.StatusOK)

	middleware.UsersClient = usersClientMock(org, []*users.User{viewer, editor, admin}, permission.UpdateNotificationSettings)
	path = fmt.Sprintf("/api/app/%s/api/notification/config/receivers/97b52281-5c6d-4250-b9a7-ad8a65427fac", org.ExternalID)

	testMiddleware(t, &middleware, viewer, "PUT", path, http.StatusBadGateway)
	testMiddleware(t, &middleware, editor, "PUT", path, http.StatusBadGateway)
	testMiddleware(t, &middleware, admin, "PUT", path, http.StatusOK)
}
