package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/weaveworks/service/common/billing/grpc"
	"github.com/weaveworks/service/common/billing/provider"
	"github.com/weaveworks/service/common/featureflag"
	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/db/dbtest"
)

func Test_Org(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, org, team := dbtest.GetOrgAndTeam(t, database)

	// Check the user was added to the org
	organizations, err := database.ListOrganizationsForUserIDs(context.Background(), user.ID)
	assert.NoError(t, err)
	require.Len(t, organizations, 1)
	assert.Equal(t, org.ID, organizations[0].ID, "organization should have an id")
	assert.Equal(t, org.ExternalID, organizations[0].ExternalID, "organization should have an external id")
	assert.Equal(t, org.Name, organizations[0].Name, "organization should have a name")
	assert.Equal(t, org.TrialExpiresAt, organizations[0].TrialExpiresAt, "organization should have a trial expiry")
	assert.NotEqual(t, "", organizations[0].ProbeToken, "organization should have a probe token")

	org, err = database.FindOrganizationByProbeToken(context.Background(), organizations[0].ProbeToken)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	r := requestAs(t, user, "GET", "/api/users/org/"+organizations[0].ExternalID, nil)
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	body := map[string]interface{}{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	trialExpiresAt, err := org.TrialExpiresAt.MarshalText()
	assert.NoError(t, err)
	assert.Equal(t, map[string]interface{}{
		"user":                  user.Email,
		"id":                    org.ExternalID,
		"name":                  org.Name,
		"probeToken":            org.ProbeToken,
		"refuseDataAccess":      org.RefuseDataAccess,
		"refuseDataUpload":      org.RefuseDataUpload,
		"firstSeenConnectedAt":  nil,
		"platform":              org.Platform,
		"platformVersion":       org.PlatformVersion,
		"environment":           org.Environment,
		"trialExpiresAt":        string(trialExpiresAt),
		"zuoraAccountNumber":    "",
		"zuoraAccountCreatedAt": nil,
		"billingProvider":       "zuora",
		"teamId":                team.ExternalID,
	}, body)
}

func Test_OrgFiltersTokenIfMissingPermission(t *testing.T) {
	setup(t)
	defer cleanup(t)

	admin, org, team := dbtest.GetOrgAndTeam(t, database)
	user := getUser(t)
	err := database.AddUserToTeam(context.TODO(), user.ID, team.ID, users.ViewerRoleID)
	assert.NoError(t, err)

	w := httptest.NewRecorder()
	r := requestAs(t, admin, "GET", "/api/users/org/"+org.ExternalID, nil)
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	body := map[string]interface{}{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.NoError(t, err)
	assert.Equal(t, org.ProbeToken, body["probeToken"])

	w = httptest.NewRecorder()
	r = requestAs(t, user, "GET", "/api/users/org/"+org.ExternalID, nil)
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	body = map[string]interface{}{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.NoError(t, err)
	assert.Equal(t, nil, body["probeToken"])
}

func Test_Org_NoProbeUpdates(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, org, team := dbtest.GetOrgAndTeam(t, database)

	w := httptest.NewRecorder()
	r := requestAs(t, user, "GET", "/api/users/org/"+org.ExternalID, nil)

	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	body := map[string]interface{}{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	trialExpiresAt, err := org.TrialExpiresAt.MarshalText()
	assert.NoError(t, err)
	assert.Equal(t, map[string]interface{}{
		"user":                  user.Email,
		"id":                    org.ExternalID,
		"name":                  org.Name,
		"probeToken":            org.ProbeToken,
		"refuseDataAccess":      org.RefuseDataAccess,
		"refuseDataUpload":      org.RefuseDataUpload,
		"firstSeenConnectedAt":  nil,
		"platform":              org.Platform,
		"platformVersion":       org.PlatformVersion,
		"environment":           org.Environment,
		"trialExpiresAt":        string(trialExpiresAt),
		"zuoraAccountNumber":    "",
		"zuoraAccountCreatedAt": nil,
		"billingProvider":       "zuora",
		"teamId":                team.ExternalID,
	}, body)
}

func Test_ListOrganizationUsers(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, org := getOrg(t)

	team := getTeam(t)
	err := database.AddUserToTeam(context.TODO(), user.ID, team.ID, users.AdminRoleID)
	assert.NoError(t, err)

	err = database.MoveOrganizationToTeam(context.TODO(), org.ExternalID, team.ExternalID, "", "")
	assert.NoError(t, err)

	us, err := database.ListTeamUsersWithRoles(context.TODO(), team.ID)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(us))
	assert.Equal(t, users.AdminRoleID, us[0].Role.ID)

	fran := getUser(t)
	fran, _, err = database.InviteUserToTeam(context.Background(), fran.Email, team.ExternalID, users.AdminRoleID)
	require.NoError(t, err)

	us, err = database.ListTeamUsersWithRoles(context.TODO(), team.ID)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(us))

	w := httptest.NewRecorder()
	r := requestAs(t, user, "GET", "/api/users/org/"+org.ExternalID+"/users", nil)

	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	responseBody := map[string]interface{}{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &responseBody))
	assert.Equal(t, map[string]interface{}{
		"users": []interface{}{
			map[string]interface{}{
				"email":  fran.Email,
				"roleId": users.AdminRoleID,
			},
			map[string]interface{}{
				"email":  user.Email,
				"self":   true,
				"roleId": users.AdminRoleID,
			},
		},
	}, responseBody)
}

const (
	orgName100  = "A Different Org Name 234567890 234567890 234567890 234567890 234567890 234567890 234567890 234567890"
	orgName101  = "A DIFFERENT ORG NAME 234567890 234567890 234567890 234567890 234567890 234567890 234567890 2345678901"
	platform    = "kubernetes"
	environment = "minikube"
)

func Test_UpdateOrganization(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, org := getOrg(t)
	otherUser := getUser(t)
	body := map[string]interface{}{"name": orgName100, "platform": platform, "environment": environment}

	// Invalid auth
	{
		w := httptest.NewRecorder()
		r := requestAs(t, otherUser, "PUT", "/api/users/org/"+org.ExternalID, jsonBody(body).Reader(t))

		app.ServeHTTP(w, r)
		assert.Equal(t, http.StatusForbidden, w.Code)

		found, err := database.FindOrganizationByProbeToken(context.Background(), org.ProbeToken)
		if assert.NoError(t, err) {
			assert.Equal(t, org.Name, found.Name)
		}
	}

	// Should 404 for not found orgs
	{
		w := httptest.NewRecorder()
		r := requestAs(t, otherUser, "PUT", "/api/users/org/not-found-org", jsonBody(body).Reader(t))

		app.ServeHTTP(w, r)
		assert.Equal(t, http.StatusNotFound, w.Code)
	}

	// Should update my org
	{
		w := httptest.NewRecorder()
		r := requestAs(t, user, "PUT", "/api/users/org/"+org.ExternalID, jsonBody(body).Reader(t))

		app.ServeHTTP(w, r)
		assert.Equal(t, http.StatusNoContent, w.Code)

		organizations, err := database.ListOrganizationsForUserIDs(context.Background(), user.ID)
		require.NoError(t, err)
		if assert.Len(t, organizations, 1) {
			assert.Equal(t, org.ID, organizations[0].ID)
			assert.Equal(t, org.ExternalID, organizations[0].ExternalID)
			assert.Equal(t, orgName100, organizations[0].Name)
			assert.Equal(t, platform, organizations[0].Platform)
			assert.Equal(t, environment, organizations[0].Environment)
		}
	}

	// Should reject rename as new name exceeds maximum size
	{
		body101 := map[string]interface{}{"name": orgName101}
		w := httptest.NewRecorder()
		r := requestAs(t, user, "PUT", "/api/users/org/"+org.ExternalID, jsonBody(body101).Reader(t))

		app.ServeHTTP(w, r)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	}
}

func TestAPI_updateOrg_moveTeam(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, org := getOrg(t)
	billingClient.EXPECT().FindBillingAccountByTeamID(gomock.Any(), gomock.Any()).
		AnyTimes().
		Return(&grpc.BillingAccount{}, nil)

	{ // move from no-team to team
		team := getTeam(t)
		err := database.AddUserToTeam(context.TODO(), user.ID, team.ID, users.AdminRoleID)
		assert.NoError(t, err)

		body := map[string]interface{}{"teamId": team.ExternalID}
		w := httptest.NewRecorder()
		r := requestAs(t, user, "PUT", "/api/users/org/"+org.ExternalID, jsonBody(body).Reader(t))

		app.ServeHTTP(w, r)
		assert.Equal(t, http.StatusNoContent, w.Code)

		org, err := database.FindOrganizationByID(context.TODO(), org.ExternalID)
		if assert.NoError(t, err) {
			assert.Equal(t, team.ExternalID, org.TeamExternalID)
		}
	}
	{ // move from team to team
		team := getTeam(t)
		err := database.AddUserToTeam(context.TODO(), user.ID, team.ID, users.AdminRoleID)
		assert.NoError(t, err)

		body := map[string]interface{}{"teamId": team.ExternalID}
		w := httptest.NewRecorder()
		r := requestAs(t, user, "PUT", "/api/users/org/"+org.ExternalID, jsonBody(body).Reader(t))

		app.ServeHTTP(w, r)
		assert.Equal(t, http.StatusNoContent, w.Code)

		org, err := database.FindOrganizationByID(context.TODO(), org.ExternalID)
		if assert.NoError(t, err) {
			assert.Equal(t, team.ExternalID, org.TeamExternalID)
		}
	}
	{ // move from team to foreign team
		team := getTeam(t)

		body := map[string]interface{}{"teamId": team.ExternalID}
		w := httptest.NewRecorder()
		r := requestAs(t, user, "PUT", "/api/users/org/"+org.ExternalID, jsonBody(body).Reader(t))

		app.ServeHTTP(w, r)
		assert.Equal(t, http.StatusForbidden, w.Code)
	}
}

func TestAPI_updateOrg_moveTeamBilledExternally(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, org, team := dbtest.GetOrgAndTeam(t, database)

	exteam := getTeam(t)
	err := database.AddUserToTeam(context.TODO(), user.ID, exteam.ID, users.AdminRoleID)
	assert.NoError(t, err)

	billingClient.EXPECT().FindBillingAccountByTeamID(gomock.Any(), &grpc.BillingAccountByTeamIDRequest{TeamID: team.ID}).
		AnyTimes().
		Return(&grpc.BillingAccount{}, nil)
	billingClient.EXPECT().FindBillingAccountByTeamID(gomock.Any(), &grpc.BillingAccountByTeamIDRequest{TeamID: exteam.ID}).
		AnyTimes().
		Return(&grpc.BillingAccount{Provider: provider.External}, nil)

	{ // move to externally billed team

		body := map[string]interface{}{"teamId": exteam.ExternalID}
		w := httptest.NewRecorder()
		r := requestAs(t, user, "PUT", "/api/users/org/"+org.ExternalID, jsonBody(body).Reader(t))

		app.ServeHTTP(w, r)
		assert.Equal(t, http.StatusNoContent, w.Code)

		org, err := database.FindOrganizationByID(context.TODO(), org.ExternalID)
		if assert.NoError(t, err) {
			assert.True(t, org.HasFeatureFlag(featureflag.NoBilling))
			assert.False(t, org.HasFeatureFlag(featureflag.Billing))
		}
	}

	{ // move back to Zuora-billed team

		billingClient.EXPECT().FindBillingAccountByTeamID(gomock.Any(), &grpc.BillingAccountByTeamIDRequest{TeamID: team.ID}).
			AnyTimes().
			Return(&grpc.BillingAccount{}, nil)

		body := map[string]interface{}{"teamId": team.ExternalID}
		w := httptest.NewRecorder()
		r := requestAs(t, user, "PUT", "/api/users/org/"+org.ExternalID, jsonBody(body).Reader(t))

		app.ServeHTTP(w, r)
		assert.Equal(t, http.StatusNoContent, w.Code)

		org, err := database.FindOrganizationByID(context.TODO(), org.ExternalID)
		if assert.NoError(t, err) {
			assert.True(t, org.HasFeatureFlag(featureflag.Billing))
			assert.False(t, org.HasFeatureFlag(featureflag.NoBilling))
		}
	}
}

func Test_ReIDOrganization_NotAllowed(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, org := getOrg(t)

	w := httptest.NewRecorder()
	r := requestAs(t, user, "PUT", "/api/users/org/"+org.ExternalID, jsonBody{"id": "my-organization"}.Reader(t))

	// All non-writeable fields are filtered out.
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusNoContent, w.Code)

	organizations, err := database.ListOrganizationsForUserIDs(context.Background(), user.ID)
	require.NoError(t, err)
	if assert.Len(t, organizations, 1) {
		assert.Equal(t, org.ID, organizations[0].ID)
		assert.Equal(t, org.ExternalID, organizations[0].ExternalID)
		assert.Equal(t, org.Name, organizations[0].Name)
	}
}

func Test_UpdateOrganization_Validation(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, org := getOrg(t)

	tests := []struct {
		name        string
		platform    string
		environment string
		errMsg      string
	}{
		{"", "", "", "name cannot be blank"},
		{"Test", "invalid", "minikube", "platform is invalid"},
		{"Test", "kubernetes", "invalid", "environment is invalid"},
		{"Test", "kubernetes", "", "environment is required with platform"},
		{"Test", "", "minikube", "platform is required with environment"},
	}

	for _, tc := range tests {
		w := httptest.NewRecorder()
		r := requestAs(t, user, "PUT", "/api/users/org/"+org.ExternalID, jsonBody{
			"name":        tc.name,
			"platform":    tc.platform,
			"environment": tc.environment,
		}.Reader(t))

		app.ServeHTTP(w, r)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), fmt.Sprintf(`{"errors":[{"message":%q}]}`, tc.errMsg))

		organizations, err := database.ListOrganizationsForUserIDs(context.Background(), user.ID)
		require.NoError(t, err)
		if assert.Len(t, organizations, 1) {
			assert.Equal(t, org.ID, organizations[0].ID)
			assert.Equal(t, org.ExternalID, organizations[0].ExternalID)
			assert.Equal(t, org.Name, organizations[0].Name)
		}
	}
}

func Test_Organization_GenerateOrgExternalID(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user := getUser(t)

	// Generate a new org id
	r := requestAs(t, user, "GET", "/api/users/generateOrgID", nil)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	body := map[string]interface{}{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.NotEqual(t, "", body["id"])

	// Check it's available
	exists, err := database.OrganizationExists(context.Background(), body["id"].(string))
	require.NoError(t, err)
	assert.False(t, exists)
}

func Test_Organization_CheckIfExternalIDExists(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user := getUser(t)
	otherUser := getUser(t)

	id, err := database.GenerateOrganizationExternalID(context.Background())
	require.NoError(t, err)

	r := requestAs(t, user, "GET", "/api/users/org/"+id, nil)

	{
		w := httptest.NewRecorder()
		app.ServeHTTP(w, r)
		assert.Equal(t, http.StatusNotFound, w.Code)
	}

	// Create the org so it exists
	org, err := database.CreateOrganizationWithTeam(context.Background(), otherUser.ID, id, id, "", "", "Some Team", user.TrialExpiresAt())
	require.NoError(t, err)

	{
		w := httptest.NewRecorder()
		app.ServeHTTP(w, r)
		assert.Equal(t, http.StatusForbidden, w.Code)
	}

	// Delete the org and check it is no longer available
	err = database.DeleteOrganization(context.Background(), org.ExternalID, user.ID)
	require.NoError(t, err)

	{
		w := httptest.NewRecorder()
		app.ServeHTTP(w, r)
		assert.Equal(t, http.StatusNotFound, w.Code)
	}
}

func Test_Organization_Delete(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user := getUser(t)
	otherUser := getUser(t)

	externalID, err := database.GenerateOrganizationExternalID(context.Background())
	require.NoError(t, err)

	r := requestAs(t, otherUser, "DELETE", "/api/users/org/"+externalID, nil)

	// Should NoContent if the org already doesn't exist
	{
		w := httptest.NewRecorder()
		app.ServeHTTP(w, r)
		assert.Equal(t, http.StatusNotFound, w.Code)
	}

	// Create the org so it exists
	org, err := database.CreateOrganizationWithTeam(context.Background(), user.ID, externalID, externalID, "", "", "Some Team", user.TrialExpiresAt())
	require.NoError(t, err)

	// Should 401 because otherUser doesn't have access
	{
		w := httptest.NewRecorder()
		app.ServeHTTP(w, r)
		assert.Equal(t, http.StatusForbidden, w.Code)
	}

	// Login as the org owner
	r = requestAs(t, user, "DELETE", "/api/users/org/"+externalID, nil)

	{
		w := httptest.NewRecorder()
		app.ServeHTTP(w, r)
		assert.Equal(t, http.StatusNoContent, w.Code)
	}

	// Check the org no longer exists
	_, err = database.FindOrganizationByID(context.Background(), org.ExternalID)
	require.EqualError(t, err, users.ErrNotFound.Error())

	// Check the user no longer has the org
	isMember, err := database.UserIsMemberOf(context.Background(), user.ID, org.ExternalID)
	require.NoError(t, err)
	assert.False(t, isMember, "Expected user not to have the deleted org any more")

	// Check org doesn't appear in listing.
	organizations, err := database.ListOrganizationsForUserIDs(context.Background(), user.ID)
	require.NoError(t, err)
	assert.Len(t, organizations, 0)

	// Check org still appears in "all" listing including deleted orgs
	organizations, err = database.ListAllOrganizationsForUserIDs(context.Background(), "", user.ID)
	require.NoError(t, err)
	assert.Len(t, organizations, 1)
}

func Test_Organization_DeleteGCP(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user := getUser(t)

	// Create the org so it exists
	org, err := database.CreateOrganizationWithGCP(context.Background(), user.ID, "FOO", user.TrialExpiresAt())
	require.NoError(t, err)

	// Login as the org owner
	r := requestAs(t, user, "DELETE", "/api/users/org/"+org.ExternalID, nil)

	{
		w := httptest.NewRecorder()
		app.ServeHTTP(w, r)
		assert.Equal(t, http.StatusForbidden, w.Code)
	}

	// Check the org still exists
	_, err = database.FindOrganizationByID(context.Background(), org.ExternalID)
	assert.NoError(t, err)
}

func Test_Organization_Name(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user := getUser(t)
	externalID, err := database.GenerateOrganizationExternalID(context.Background())
	require.NoError(t, err)

	_, err = database.CreateOrganizationWithTeam(context.Background(), user.ID, externalID, orgName100, "", "", "Some Team", user.TrialExpiresAt())
	require.NoError(t, err)

	foundName, err := database.GetOrganizationName(context.Background(), externalID)
	assert.Equal(t, orgName100, foundName)
}

func Test_Organization_Overlong_Name(t *testing.T) {
	setup(t)
	defer cleanup(t)
	{
		user := getUser(t)
		externalID, err := database.GenerateOrganizationExternalID(context.Background())
		require.NoError(t, err)

		_, err = database.CreateOrganizationWithTeam(context.Background(), user.ID, externalID, orgName101, "", "", "Some Team", user.TrialExpiresAt())
		assert.IsType(t, &pq.Error{}, err)
	}
}

func Test_Organization_Lookup(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user := getUser(t)
	org := createOrgForUser(t, user)

	request, err := http.NewRequest("GET", "/api/users/org/lookup", nil)
	require.NoError(t, err)
	request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", org.ProbeToken))

	w := httptest.NewRecorder()
	app.ServeHTTP(w, request)
	assert.Equal(t, http.StatusOK, w.Code)

	body := map[string]interface{}{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, org.ExternalID, body["externalID"])
	assert.Equal(t, org.Name, body["name"])
}

func Test_Organization_UpdatePlatformVersion(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, org, _ := dbtest.GetOrgAndTeam(t, database)

	bytes := doRequest(t, user, "GET", "/api/users/org/"+org.ExternalID, nil, http.StatusOK)
	body := map[string]interface{}{}
	assert.NoError(t, json.Unmarshal(bytes, &body))
	assert.Equal(t, "", body["platformVersion"])

	body = map[string]interface{}{"platformVersion": "redredred"}
	request, err := http.NewRequest("PUT", "/api/users/org/platform_version", jsonBody(body).Reader(t))
	require.NoError(t, err)
	request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", org.ProbeToken))
	w := httptest.NewRecorder()
	app.ServeHTTP(w, request)
	assert.Equal(t, http.StatusNoContent, w.Code)

	bytes = doRequest(t, user, "GET", "/api/users/org/"+org.ExternalID, nil, http.StatusOK)
	body = map[string]interface{}{}
	assert.NoError(t, json.Unmarshal(bytes, &body))
	assert.Equal(t, "redredred", body["platformVersion"])

}
