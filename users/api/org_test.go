package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weaveworks/service/common/billing/grpc"
	"github.com/weaveworks/service/common/billing/provider"
	"github.com/weaveworks/service/common/featureflag"
	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/api"
)

func Test_Org(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, org := getOrg(t)

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
		"environment":           org.Environment,
		"trialExpiresAt":        string(trialExpiresAt),
		"zuoraAccountNumber":    "",
		"zuoraAccountCreatedAt": nil,
		"billingProvider":       "zuora",
	}, body)
}

func Test_Org_NoProbeUpdates(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, org := getOrg(t)

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
		"environment":           org.Environment,
		"trialExpiresAt":        string(trialExpiresAt),
		"zuoraAccountNumber":    "",
		"zuoraAccountCreatedAt": nil,
		"billingProvider":       "zuora",
	}, body)
}

func Test_ListOrganizationUsers(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, org := getOrg(t)

	fran := getUser(t)
	fran, _, err := database.InviteUser(context.Background(), fran.Email, org.ExternalID)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	r := requestAs(t, user, "GET", "/api/users/org/"+org.ExternalID+"/users", nil)

	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), fmt.Sprintf(`{"users":[{"email":%q},{"email":%q,"self":true}]}`, fran.Email, user.Email))
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
		err := database.AddUserToTeam(context.TODO(), user.ID, team.ID)
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
		err := database.AddUserToTeam(context.TODO(), user.ID, team.ID)
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

	user, org := getOrg(t)

	exteam := getTeam(t)
	err := database.AddUserToTeam(context.TODO(), user.ID, exteam.ID)
	assert.NoError(t, err)

	billingClient.EXPECT().FindBillingAccountByTeamID(gomock.Any(), &grpc.BillingAccountByTeamIDRequest{TeamID: ""}).
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
		team := getTeam(t)
		err := database.AddUserToTeam(context.TODO(), user.ID, team.ID)
		assert.NoError(t, err)

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

func Test_CustomExternalIDOrganization(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user := getUser(t)
	billingClient.EXPECT().FindBillingAccountByTeamID(gomock.Any(), gomock.Any()).
		AnyTimes().
		Return(&grpc.BillingAccount{}, nil)

	w := httptest.NewRecorder()
	r := requestAs(t, user, "POST", "/api/users/org", jsonBody{
		"id":       "my-organization",
		"name":     "my organization",
		"teamName": "crew",
	}.Reader(t))

	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusCreated, w.Code)

	organizations, err := database.ListOrganizationsForUserIDs(context.Background(), user.ID)
	require.NoError(t, err)
	if assert.Len(t, organizations, 1) {
		assert.NotEqual(t, "", organizations[0].ID)
		assert.Equal(t, "my-organization", organizations[0].ExternalID)
		assert.Equal(t, "my organization", organizations[0].Name)
	}
}

func Test_CustomExternalIDOrganization_Validation(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, otherOrg := getOrg(t)

	for id, errMsg := range map[string]string{
		"": "ID cannot be blank",
		"org with^/invalid&characters": "ID can only contain letters, numbers, hyphen, and underscore",
		otherOrg.ExternalID:            "ID is already taken",
	} {
		w := httptest.NewRecorder()
		r := requestAs(t, user, "POST", "/api/users/org", jsonBody{
			"id":       id,
			"name":     "my organization",
			"teamName": "crew",
		}.Reader(t))

		app.ServeHTTP(w, r)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), fmt.Sprintf(`{"errors":[{"message":%q}]}`, errMsg))

		organizations, err := database.ListOrganizationsForUserIDs(context.Background(), user.ID)
		require.NoError(t, err)
		if assert.Len(t, organizations, 1) {
			assert.Equal(t, otherOrg.ID, organizations[0].ID)
			assert.Equal(t, otherOrg.ExternalID, organizations[0].ExternalID)
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
	org, err := database.CreateOrganization(context.Background(), otherUser.ID, id, id, "", "", user.TrialExpiresAt())
	require.NoError(t, err)

	{
		w := httptest.NewRecorder()
		app.ServeHTTP(w, r)
		assert.Equal(t, http.StatusForbidden, w.Code)
	}

	// Delete the org and check it is no longer available
	err = database.DeleteOrganization(context.Background(), org.ExternalID)
	require.NoError(t, err)

	{
		w := httptest.NewRecorder()
		app.ServeHTTP(w, r)
		assert.Equal(t, http.StatusNotFound, w.Code)
	}
}

func Test_Organization_CreateMultiple(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user := getUser(t)
	billingClient.EXPECT().FindBillingAccountByTeamID(gomock.Any(), gomock.Any()).
		AnyTimes().
		Return(&grpc.BillingAccount{}, nil)

	r1 := requestAs(t, user, "POST", "/api/users/org", jsonBody{"id": "my-first-org", "name": "my first org", "teamName": "crew"}.Reader(t))

	w := httptest.NewRecorder()
	app.ServeHTTP(w, r1)
	assert.Equal(t, http.StatusCreated, w.Code)

	r2 := requestAs(t, user, "POST", "/api/users/org", jsonBody{"id": "my-second-org", "name": "my second org", "teamName": "squad"}.Reader(t))

	w = httptest.NewRecorder()
	app.ServeHTTP(w, r2)
	assert.Equal(t, http.StatusCreated, w.Code)

	organizations, err := database.ListOrganizationsForUserIDs(context.Background(), user.ID)
	require.NoError(t, err)
	if assert.Len(t, organizations, 2) {
		assert.NotEqual(t, "", organizations[0].ID)
		assert.Equal(t, "my-second-org", organizations[0].ExternalID)
		assert.Equal(t, "my second org", organizations[0].Name)
		assert.NotEqual(t, "", organizations[1].ID)
		assert.Equal(t, "my-first-org", organizations[1].ExternalID)
		assert.Equal(t, "my first org", organizations[1].Name)
	}
}

func Test_Organization_CreateOrg_delinquent(t *testing.T) {
	setup(t)
	defer cleanup(t)

	now := time.Date(2022, 6, 27, 0, 0, 0, 0, time.UTC)
	user := getUser(t)
	eid := "delinquent-refuse-12"

	billingClient.EXPECT().FindBillingAccountByTeamID(gomock.Any(), gomock.Any()).
		AnyTimes().
		Return(&grpc.BillingAccount{}, nil)
	err := app.CreateOrg(context.TODO(), user, api.OrgView{
		ExternalID:     eid,
		Name:           "name",
		TrialExpiresAt: now.Add(-17 * 24 * time.Hour), // more than 15d back to trigger upload refusal
		TeamName:       "crew",
	}, now)
	assert.NoError(t, err)

	org, err := database.FindOrganizationByID(context.TODO(), eid)
	if assert.NoError(t, err) {
		assert.True(t, org.RefuseDataAccess)
		assert.True(t, org.RefuseDataUpload)
	}
}

func Test_Organization_CreateOrg_never_delinquent_for_weaver(t *testing.T) {
	setup(t)
	defer cleanup(t)

	now := time.Date(2022, 6, 27, 0, 0, 0, 0, time.UTC)
	user := getUserWithDomain(t, "weave.works")
	eid := "delinquent-accept-12"

	billingClient.EXPECT().FindBillingAccountByTeamID(gomock.Any(), gomock.Any()).
		AnyTimes().
		Return(&grpc.BillingAccount{}, nil)
	err := app.CreateOrg(context.TODO(), user, api.OrgView{
		ExternalID:     eid,
		Name:           "name",
		TrialExpiresAt: now.Add(-17 * 24 * time.Hour), // more than 15d back to (normally) trigger upload refusal
		TeamName:       "crew",
	}, now)
	assert.NoError(t, err)

	org, err := database.FindOrganizationByID(context.TODO(), eid)
	assert.NoError(t, err)
	assert.Contains(t, org.FeatureFlags, featureflag.NoBilling)
	assert.False(t, org.RefuseDataAccess)
	assert.False(t, org.RefuseDataUpload)
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
		assert.Equal(t, http.StatusNoContent, w.Code)
	}

	// Create the org so it exists
	org, err := database.CreateOrganization(context.Background(), user.ID, externalID, externalID, "", "", user.TrialExpiresAt())
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

	_, err = database.CreateOrganization(context.Background(), user.ID, externalID, orgName100, "", "", user.TrialExpiresAt())
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

		_, err = database.CreateOrganization(context.Background(), user.ID, externalID, orgName101, "", "", user.TrialExpiresAt())
		assert.IsType(t, &pq.Error{}, err)
	}
}

func Test_Organization_CreateTeam(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user := getUser(t)

	billingClient.EXPECT().FindBillingAccountByTeamID(gomock.Any(), gomock.Any()).Return(&grpc.BillingAccount{}, nil)

	teamName := "my-team-name"
	r1 := requestAs(t, user, "POST", "/api/users/org", jsonBody{"id": "my-org-id", "name": "my-org-name", "teamName": teamName}.Reader(t))

	w := httptest.NewRecorder()
	app.ServeHTTP(w, r1)
	assert.Equal(t, http.StatusCreated, w.Code)

	organizations, err := database.ListOrganizationsForUserIDs(context.Background(), user.ID)
	require.NoError(t, err)
	assert.Len(t, organizations, 1)
	assert.NotEqual(t, "", organizations[0].ID)
	assert.Equal(t, "my-org-id", organizations[0].ExternalID)
	assert.Equal(t, "my-org-name", organizations[0].Name)
	assert.NotEqual(t, "", organizations[0].TeamID)
	assert.NotEqual(t, "", organizations[0].TeamExternalID)

	r2 := requestAs(t, user, "GET", "/api/users/teams", nil)
	w = httptest.NewRecorder()
	app.ServeHTTP(w, r2)
	assert.Equal(t, http.StatusOK, w.Code)
	body := api.TeamsView{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))

	assert.Len(t, body.Teams, 1)
	assert.Equal(t, body.Teams[0].ExternalID, organizations[0].TeamExternalID)
	assert.Equal(t, body.Teams[0].Name, teamName)
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

func Test_Organization_UserWithExpiredTrialButInTeamBilledExternallyShouldBeAbleToAccessOrganization(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user := getUser(t)

	// Create a team, and simulate the fact that team is marked as "billed externally":
	team := getTeam(t)
	err := database.AddUserToTeam(context.Background(), user.ID, team.ID)
	assert.NoError(t, err)
	billingClient.EXPECT().FindBillingAccountByTeamID(gomock.Any(), &grpc.BillingAccountByTeamIDRequest{TeamID: team.ID}).
		Return(&grpc.BillingAccount{Provider: provider.External}, nil)

	// Simulate the fact that the organization we're about to create is created by an user whose trial is already expired, i.e. the organization is delinquent:
	trialExpiresAt := time.Now().Add(-17 * 24 * time.Hour) // more than 15d back to trigger upload refusal

	err = app.CreateOrg(context.Background(), user, api.OrgView{
		ExternalID:     "my-org-id",
		Name:           "my-org-name",
		TeamExternalID: team.ExternalID,
		TrialExpiresAt: trialExpiresAt,
	}, time.Now())
	assert.NoError(t, err)

	orgs, err := database.ListOrganizationsForUserIDs(context.Background(), user.ID)
	require.NoError(t, err)
	assert.Len(t, orgs, 1)
	org := orgs[0]
	assert.NotEqual(t, "", org.ID)
	assert.Equal(t, "my-org-id", org.ExternalID)
	assert.Equal(t, "my-org-name", org.Name)
	assert.NotEqual(t, "", org.TeamID)
	assert.NotEqual(t, "", org.TeamExternalID)
	assert.NotContains(t, org.FeatureFlags, featureflag.Billing)
	assert.Contains(t, org.FeatureFlags, featureflag.NoBilling)
	assert.False(t, org.RefuseDataAccess)
	assert.False(t, org.RefuseDataUpload)
}

func Test_Organization_UserWithExpiredTrialAndNotInTeamBilledExternallyShouldNotBeAbleToAccessOrganization(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user := getUser(t)

	// Create a team, and simulate the fact that team is NOT marked as "billed externally":
	team := getTeam(t)
	err := database.AddUserToTeam(context.Background(), user.ID, team.ID)
	assert.NoError(t, err)
	billingClient.EXPECT().FindBillingAccountByTeamID(gomock.Any(), &grpc.BillingAccountByTeamIDRequest{TeamID: team.ID}).
		Return(&grpc.BillingAccount{}, nil)

	// Simulate the fact that the organization we're about to create is created by an user whose trial is already expired, i.e. the organization is delinquent:
	trialExpiresAt := time.Now().Add(-17 * 24 * time.Hour) // more than 15d back to trigger upload refusal

	err = app.CreateOrg(context.Background(), user, api.OrgView{
		ExternalID:     "my-org-id",
		Name:           "my-org-name",
		TeamExternalID: team.ExternalID,
		TrialExpiresAt: trialExpiresAt,
	}, time.Now())
	assert.NoError(t, err)

	orgs, err := database.ListOrganizationsForUserIDs(context.Background(), user.ID)
	require.NoError(t, err)
	assert.Len(t, orgs, 1)
	org := orgs[0]
	assert.NotEqual(t, "", org.ID)
	assert.Equal(t, "my-org-id", org.ExternalID)
	assert.Equal(t, "my-org-name", org.Name)
	assert.NotEqual(t, "", org.TeamID)
	assert.NotEqual(t, "", org.TeamExternalID)
	assert.NotContains(t, org.FeatureFlags, featureflag.NoBilling)
	assert.Contains(t, org.FeatureFlags, featureflag.Billing)
	assert.True(t, org.RefuseDataAccess)
	assert.True(t, org.RefuseDataUpload)
}
