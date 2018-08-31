package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/weaveworks/service/common/featureflag"
	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/client"
	"github.com/weaveworks/service/users/db/dbtest"
)

var (
	ghToken   = "e12eb509a297f56dcc77c86ec9e44369080698a6"
	ghSession = []byte(`{"token": {"expiry": "0001-01-01T00:00:00Z", "token_type": "bearer", "access_token": "` + ghToken + `"}}`)
	ctx       = context.Background()
)

func TestAPI_adminChangeOrgFields(t *testing.T) {
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

func TestAPI_adminChangeOrgFields_BillingFeatureFlags(t *testing.T) {
	setup(t)
	defer cleanup(t)

	_, org := dbtest.GetOrg(t, database)

	// Set trial expiration to trigger extension and notifications
	prevExpires := time.Now()
	_, err := database.UpdateOrganization(ctx, org.ExternalID, users.OrgWriteView{TrialExpiresAt: &prevExpires})
	assert.NoError(t, err)

	assert.False(t, org.HasFeatureFlag(featureflag.Billing))

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
	assert.True(t, newOrg.HasFeatureFlag(featureflag.Billing))
	assert.True(t, newOrg.HasFeatureFlag("foo"))
	assert.True(t, newOrg.HasFeatureFlag("moo"))
}

func TestAPI_adminChangeOrgFields_BillingNeverShrinkTrialPeriod(t *testing.T) {
	setup(t)
	defer cleanup(t)

	_, org := dbtest.GetOrg(t, database)

	// way in the future to make sure a possible extension does not go past it
	prevExpires := time.Now().Add(12 * 30 * 24 * time.Hour).Truncate(1 * time.Second)
	_, err := database.UpdateOrganization(ctx, org.ExternalID, users.OrgWriteView{TrialExpiresAt: &prevExpires})
	assert.NoError(t, err)

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

func TestAPI_adminGetUserToken(t *testing.T) {
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

func TestAPI_adminGetUserToken_NoUser(t *testing.T) {
	setup(t)
	defer cleanup(t)

	w := httptest.NewRecorder()
	r, err := http.NewRequest("GET",
		fmt.Sprintf("%s/admin/users/users/%v/logins/github/token", domain, "unknown"), nil)
	app.ServeHTTP(w, r)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAPI_adminGetUserToken_NoToken(t *testing.T) {
	setup(t)
	defer cleanup(t)

	usr, _ := database.CreateUser(ctx, "test@test")

	w := httptest.NewRecorder()
	r := requestAs(t, usr, "GET",
		fmt.Sprintf("/admin/users/users/%v/logins/github/token", usr.ID), nil)
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAPI_adminDeleteUser(t *testing.T) {
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

func TestAPI_adminTrial(t *testing.T) {
	setup(t)
	defer cleanup(t)

	usr, org := getOrg(t)
	assert.Equal(t, 14, org.TrialRemaining(), "trial is not 14 days on instance creation")

	{ // Cannot shrink
		w := httptest.NewRecorder()
		r := requestAs(t, usr, "POST", fmt.Sprintf("/admin/users/organizations/%s/trial", org.ExternalID), strings.NewReader("remaining=9"))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		app.ServeHTTP(w, r)
		assert.Equal(t, http.StatusFound, w.Code)

		org, err := database.FindOrganizationByID(context.TODO(), org.ExternalID)
		assert.NoError(t, err)
		assert.Equal(t, 14, org.TrialRemaining())
	}
	{ // but can expand
		w := httptest.NewRecorder()
		r := requestAs(t, usr, "POST", fmt.Sprintf("/admin/users/organizations/%s/trial", org.ExternalID), strings.NewReader("remaining=31"))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		app.ServeHTTP(w, r)
		assert.Equal(t, http.StatusFound, w.Code)

		org, err := database.FindOrganizationByID(context.TODO(), org.ExternalID)
		assert.NoError(t, err)
		assert.Equal(t, 31, org.TrialRemaining())
	}
	{ // resets TrialExpiredNotifiedAt
		// set it to already notified
		expires := time.Now().UTC().Add(-5 * 24 * time.Hour)
		notified := expires.Add(24 * time.Hour)
		_, err := database.UpdateOrganization(ctx, org.ExternalID, users.OrgWriteView{
			TrialExpiresAt:         &expires,
			TrialExpiredNotifiedAt: &notified,
		})
		assert.NoError(t, err)
		org, err := database.FindOrganizationByID(context.TODO(), org.ExternalID)
		assert.NoError(t, err)
		assert.NotNil(t, org.TrialExpiredNotifiedAt)

		w := httptest.NewRecorder()
		r := requestAs(t, usr, "POST", fmt.Sprintf("/admin/users/organizations/%s/trial", org.ExternalID), strings.NewReader("remaining=3"))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		app.ServeHTTP(w, r)
		assert.Equal(t, http.StatusFound, w.Code)

		org, err = database.FindOrganizationByID(context.TODO(), org.ExternalID)
		assert.NoError(t, err)
		assert.Nil(t, org.TrialExpiredNotifiedAt)
	}
}
