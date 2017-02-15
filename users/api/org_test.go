package api_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"
)

func Test_Org(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, org := getOrg(t)

	// Check the user was added to the org
	organizations, err := database.ListOrganizationsForUserIDs(context.Background(), user.ID)
	assert.NoError(t, err)
	require.Len(t, organizations, 1)
	assert.Equal(t, org.ID, organizations[0].ID, "user should have an organization id")
	assert.Equal(t, org.ExternalID, organizations[0].ExternalID, "user should have an organization external id")
	assert.Equal(t, org.Name, organizations[0].Name, "user should have an organization name")
	assert.NotEqual(t, "", organizations[0].ProbeToken, "user should have a probe token")

	org, err = database.FindOrganizationByProbeToken(context.Background(), organizations[0].ProbeToken)
	require.NoError(t, err)
	require.NotNil(t, org.FirstProbeUpdateAt)

	w := httptest.NewRecorder()
	r := requestAs(t, user, "GET", "/api/users/org/"+organizations[0].ExternalID, nil)
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	body := map[string]interface{}{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, map[string]interface{}{
		"user":               user.Email,
		"id":                 org.ExternalID,
		"name":               org.Name,
		"probeToken":         org.ProbeToken,
		"firstProbeUpdateAt": org.FirstProbeUpdateAt.UTC().Format(time.RFC3339),
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
	assert.Equal(t, map[string]interface{}{
		"user":       user.Email,
		"id":         org.ExternalID,
		"name":       org.Name,
		"probeToken": org.ProbeToken,
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
	assert.Contains(t, w.Body.String(), fmt.Sprintf(`{"users":[{"email":%q,"self":true},{"email":%q}]}`, user.Email, fran.Email))
}

func Test_RenameOrganization(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, org := getOrg(t)
	otherUser := getUser(t)
	body := map[string]interface{}{"name": "my-organization"}

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
			assert.Equal(t, "my-organization", organizations[0].Name)
		}
	}
}

func Test_ReIDOrganization_NotAllowed(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, org := getOrg(t)

	w := httptest.NewRecorder()
	r := requestAs(t, user, "PUT", "/api/users/org/"+org.ExternalID, jsonBody{"id": "my-organization"}.Reader(t))

	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), `{"errors":[{"message":"ID cannot be changed"}]}`)

	organizations, err := database.ListOrganizationsForUserIDs(context.Background(), user.ID)
	require.NoError(t, err)
	if assert.Len(t, organizations, 1) {
		assert.Equal(t, org.ID, organizations[0].ID)
		assert.Equal(t, org.ExternalID, organizations[0].ExternalID)
		assert.Equal(t, org.Name, organizations[0].Name)
	}
}

func Test_RenameOrganization_Validation(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, org := getOrg(t)

	for name, errMsg := range map[string]string{
		"": "Name cannot be blank",
	} {
		w := httptest.NewRecorder()
		r := requestAs(t, user, "PUT", "/api/users/org/"+org.ExternalID, jsonBody{"name": name}.Reader(t))

		app.ServeHTTP(w, r)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), fmt.Sprintf(`{"errors":[{"message":%q}]}`, errMsg))

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

	w := httptest.NewRecorder()
	r := requestAs(t, user, "POST", "/api/users/org", jsonBody{
		"id":   "my-organization",
		"name": "my organization",
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
			"id":   id,
			"name": "my organization",
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
	body := map[string]string{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.NotEqual(t, "", body["id"])

	// Check it's available
	exists, err := database.OrganizationExists(context.Background(), body["id"])
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
	_, err = database.CreateOrganization(context.Background(), otherUser.ID, id, id, "")
	require.NoError(t, err)

	{
		w := httptest.NewRecorder()
		app.ServeHTTP(w, r)
		assert.Equal(t, http.StatusForbidden, w.Code)
	}
}

func Test_Organization_CreateMultiple(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user := getUser(t)

	r1 := requestAs(t, user, "POST", "/api/users/org", jsonBody{"id": "my-first-org", "name": "my first org"}.Reader(t))

	w := httptest.NewRecorder()
	app.ServeHTTP(w, r1)
	assert.Equal(t, http.StatusCreated, w.Code)

	r2 := requestAs(t, user, "POST", "/api/users/org", jsonBody{"id": "my-second-org", "name": "my second org"}.Reader(t))

	w = httptest.NewRecorder()
	app.ServeHTTP(w, r2)
	assert.Equal(t, http.StatusCreated, w.Code)

	organizations, err := database.ListOrganizationsForUserIDs(context.Background(), user.ID)
	require.NoError(t, err)
	if assert.Len(t, organizations, 2) {
		assert.NotEqual(t, "", organizations[0].ID)
		assert.Equal(t, "my-first-org", organizations[0].ExternalID)
		assert.Equal(t, "my first org", organizations[0].Name)
		assert.NotEqual(t, "", organizations[1].ID)
		assert.Equal(t, "my-second-org", organizations[1].ExternalID)
		assert.Equal(t, "my second org", organizations[1].Name)
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
		assert.Equal(t, http.StatusNoContent, w.Code)
	}

	// Create the org so it exists
	org, err := database.CreateOrganization(context.Background(), user.ID, externalID, externalID, "")
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
	exists, err := database.OrganizationExists(context.Background(), org.ExternalID)
	require.NoError(t, err)
	require.False(t, exists)

	// Check the user no longer has the org
	isMember, err := database.UserIsMemberOf(context.Background(), user.ID, org.ExternalID)
	require.NoError(t, err)
	assert.False(t, isMember, "Expected user not to have the deleted org any more")
}

func Test_Organization_Name(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user := getUser(t)
	externalID, err := database.GenerateOrganizationExternalID(context.Background())
	name := "arbitrary name"
	require.NoError(t, err)

	_, err = database.CreateOrganization(context.Background(), user.ID, externalID, name, "")
	require.NoError(t, err)

	foundName, err := database.GetOrganizationName(context.Background(), externalID)
	assert.Equal(t, name, foundName)
}
