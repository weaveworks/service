package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/client"
	"github.com/weaveworks/service/users/db/dbtest"
)

var (
	ghToken   = "e12eb509a297f56dcc77c86ec9e44369080698a6"
	ghSession = []byte(`{"token": {"expiry": "0001-01-01T00:00:00Z", "token_type": "bearer", "access_token": "` + ghToken + `"}}`)
	ctx       = context.Background()
)

func TestAPI_ChangeOrgFields(t *testing.T) {
	setup(t)
	defer cleanup(t)

	_, org := dbtest.GetOrg(t, database)

	ts := httptest.NewServer(app.Handler)
	r, err := http.PostForm(
		fmt.Sprintf("%s/admin/users/organizations/%s", ts.URL, org.ExternalID),
		url.Values{"FeatureFlags": {"foo"}, "RefuseDataAccess": {"on"}, "RefuseDataUpload": {"on"}},
	)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, r.StatusCode)

	assert.Len(t, sentEmails, 0)
	newOrg, _ := database.FindOrganizationByID(ctx, org.ExternalID)
	assert.True(t, newOrg.HasFeatureFlag("foo"))
	assert.True(t, newOrg.RefuseDataAccess)
	assert.True(t, newOrg.RefuseDataUpload)
}

func TestAPI_ChangeOrgFields_BillingFeatureFlags(t *testing.T) {
	setup(t)
	defer cleanup(t)

	_, org := dbtest.GetOrg(t, database)

	// Set trial expiration to trigger extension and notifications
	prevExpires := time.Now()
	database.UpdateOrganization(ctx, org.ExternalID, users.OrgWriteView{TrialExpiresAt: &prevExpires})

	assert.False(t, org.HasFeatureFlag("billing"))

	ts := httptest.NewServer(app.Handler)
	r, err := http.PostForm(
		fmt.Sprintf("%s/admin/users/organizations/%s", ts.URL, org.ExternalID),
		url.Values{"FeatureFlags": {"foo billing moo"}},
	)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, r.StatusCode)

	assert.Len(t, sentEmails, 1)

	newOrg, _ := database.FindOrganizationByID(ctx, org.ExternalID)
	assert.True(t, prevExpires.Before(newOrg.TrialExpiresAt))
	assert.True(t, newOrg.HasFeatureFlag("billing"))
	assert.True(t, newOrg.HasFeatureFlag("foo"))
	assert.True(t, newOrg.HasFeatureFlag("moo"))
}

func TestAPI_ChangeOrgFields_BillingNeverShrinkTrialPeriod(t *testing.T) {
	setup(t)
	defer cleanup(t)

	_, org := dbtest.GetOrg(t, database)

	// way in the future to make sure a possible extension does not go past it
	prevExpires := time.Now().Add(12 * 30 * 24 * time.Hour).Truncate(1 * time.Second)
	database.UpdateOrganization(ctx, org.ExternalID, users.OrgWriteView{TrialExpiresAt: &prevExpires})

	ts := httptest.NewServer(app.Handler)
	r, err := http.PostForm(
		fmt.Sprintf("%s/admin/users/organizations/%s", ts.URL, org.ExternalID),
		url.Values{"FeatureFlags": {"foo billing moo"}},
	)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, r.StatusCode)

	assert.Len(t, sentEmails, 0)
	newOrg, _ := database.FindOrganizationByID(ctx, org.ExternalID)
	assert.True(t, prevExpires.Equal(newOrg.TrialExpiresAt))
}

func TestAPI_GetUserToken(t *testing.T) {
	setup(t)
	defer cleanup(t)

	usr, _ := database.CreateUser(ctx, "test@test")
	database.AddLoginToUser(ctx, usr.ID, "github", "12345", ghSession)

	w := httptest.NewRecorder()
	r := requestAs(t, usr, "GET",
		fmt.Sprintf("/admin/users/users/%v/logins/github/token", usr.ID), nil)
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)

	var tok client.ProviderToken
	err := json.NewDecoder(w.Body).Decode(&tok)
	assert.NoError(t, err)
	assert.Equal(t, ghToken, tok.Token)
}

func TestAPI_GetUserToken_NoUser(t *testing.T) {
	setup(t)
	defer cleanup(t)

	w := httptest.NewRecorder()
	r, err := http.NewRequest("GET",
		fmt.Sprintf("%s/admin/users/users/%v/logins/github/token", domain, "unknown"), nil)
	app.ServeHTTP(w, r)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAPI_GetUserToken_NoToken(t *testing.T) {
	setup(t)
	defer cleanup(t)

	usr, _ := database.CreateUser(ctx, "test@test")

	w := httptest.NewRecorder()
	r := requestAs(t, usr, "GET",
		fmt.Sprintf("/admin/users/users/%v/logins/github/token", usr.ID), nil)
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAPI_DeleteUser(t *testing.T) {
	setup(t)
	defer cleanup(t)

	usr, single := getOrg(t)
	other, multi := getOrg(t)
	// single: usr / multi: usr, other
	_, _, err := database.InviteUser(context.TODO(), usr.Email, multi.ExternalID)
	assert.NoError(t, err)

	{ // delete first user
		w := httptest.NewRecorder()
		r := requestAs(t, usr, "POST", fmt.Sprintf("/admin/users/users/%s/remove", usr.ID), nil)
		app.ServeHTTP(w, r)
		assert.Equal(t, http.StatusFound, w.Code)

		_, err = database.FindOrganizationByID(context.TODO(), single.ExternalID)
		assert.Equal(t, users.ErrNotFound, err)

		_, err = database.FindOrganizationByID(context.TODO(), multi.ExternalID)
		assert.NoError(t, err)
	}

	{ // delete other user
		w := httptest.NewRecorder()
		r := requestAs(t, usr, "POST", fmt.Sprintf("/admin/users/users/%s/remove", other.ID), nil)
		app.ServeHTTP(w, r)
		assert.Equal(t, http.StatusFound, w.Code)

		_, err = database.FindOrganizationByID(context.TODO(), multi.ExternalID)
		assert.Equal(t, users.ErrNotFound, err)
	}

	{ // delete already deleted user does not error
		w := httptest.NewRecorder()
		r := requestAs(t, usr, "POST", "/admin/users/users/nope/remove", nil)
		app.ServeHTTP(w, r)
		assert.Equal(t, http.StatusFound, w.Code)
	}
}
