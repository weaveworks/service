package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/weaveworks/service/users/client"
	"github.com/weaveworks/service/users/db/dbtest"
)

var (
	ghToken   = "e12eb509a297f56dcc77c86ec9e44369080698a6"
	ghSession = []byte(`{"token": {"expiry": "0001-01-01T00:00:00Z", "token_type": "bearer", "access_token": "` + ghToken + `"}}`)
	ctx       = context.Background()
)

func TestAPI_ChangeOrgField_FeatureFlags(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, org := dbtest.GetOrg(t, database)

	prevExpires := org.TrialExpiresAt
	assert.False(t, org.HasFeatureFlag("billing"))

	ts := httptest.NewServer(app.Handler)
	r, err := http.PostForm(
		fmt.Sprintf("%s/admin/users/organizations/%s", ts.URL, org.ExternalID),
		url.Values{"field": {"FeatureFlags"}, "value": {"foo billing moo"}},
	)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, r.StatusCode)

	assert.Len(t, sentEmails, 1)
	assert.Equal(t, user.Email, sentEmails[0].To[0])

	newOrg, _ := database.FindOrganizationByID(ctx, org.ExternalID)
	assert.True(t, prevExpires.Before(newOrg.TrialExpiresAt))
	newOrg.HasFeatureFlag("billing")
	newOrg.HasFeatureFlag("foo")
	newOrg.HasFeatureFlag("moo")
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
