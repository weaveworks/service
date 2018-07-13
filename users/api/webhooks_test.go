package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/weaveworks/service/common/constants/webhooks"
	"github.com/weaveworks/service/users/db/dbtest"
)

func TestAPI_listOrganizationWebhooks(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, org, _ := dbtest.GetOrgAndTeam(t, database)
	w1 := dbtest.CreateWebhookForOrg(t, database, org, webhooks.GithubPushIntegrationType)
	w2 := dbtest.CreateWebhookForOrg(t, database, org, webhooks.GithubPushIntegrationType)

	w := httptest.NewRecorder()
	r := requestAs(t, user, "GET", "/api/users/org/"+org.ExternalID+"/webhooks", nil)
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)

	body := []map[string]interface{}{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Len(t, body, 2)

	assert.NotContains(t, body[0], "ID")             // Internal ID not exposed to user.
	assert.NotContains(t, body[0], "OrganizationID") // Internal Org ID not exposed to user.
	assert.Equal(t, w1.IntegrationType, body[0]["integrationType"])
	assert.Equal(t, w1.SecretID, body[0]["secretID"])
	assert.Equal(t, w1.SecretSigningKey, body[0]["secretSigningKey"])
	assert.NotEmpty(t, body[0]["createdAt"])
	assert.Empty(t, body[0]["deletedAt"])
	assert.Empty(t, body[0]["firstSeenAt"])

	assert.NotContains(t, body[1], "ID")             // Internal ID not exposed to user.
	assert.NotContains(t, body[1], "OrganizationID") // Internal Org ID not exposed to user.
	assert.Equal(t, w2.IntegrationType, body[1]["integrationType"])
	assert.Equal(t, w2.SecretID, body[1]["secretID"])
	assert.Equal(t, w2.SecretSigningKey, body[1]["secretSigningKey"])
	assert.NotEmpty(t, body[1]["createdAt"])
	assert.Empty(t, body[1]["deletedAt"])
	assert.Empty(t, body[1]["firstSeenAt"])
}

func TestAPI_createOrganizationWebhook(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, org, _ := dbtest.GetOrgAndTeam(t, database)

	payload, err := json.Marshal(map[string]string{"integrationType": webhooks.GithubPushIntegrationType})
	assert.NoError(t, err)
	w := httptest.NewRecorder()
	r := requestAs(t, user, "POST", "/api/users/org/"+org.ExternalID+"/webhooks", bytes.NewReader(payload))
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusCreated, w.Code)

	body := map[string]interface{}{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))

	assert.NotContains(t, body, "ID")             // Internal ID not exposed to user.
	assert.NotContains(t, body, "OrganizationID") // Internal Org ID not exposed to user.
	assert.Equal(t, webhooks.GithubPushIntegrationType, body["integrationType"])
	assert.NotEmpty(t, body["secretID"])
	assert.NotEmpty(t, body["secretSigningKey"])
	assert.NotEmpty(t, body["createdAt"])
	assert.Empty(t, body["deletedAt"])
	assert.Empty(t, body["firstSeenAt"])

	// Test invalid integrationType
	payload, err = json.Marshal(map[string]string{"integrationType": "invalid"})
	assert.NoError(t, err)
	w = httptest.NewRecorder()
	r = requestAs(t, user, "POST", "/api/users/org/"+org.ExternalID+"/webhooks", bytes.NewReader(payload))
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAPI_deleteOrganizationWebhook(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, org, _ := dbtest.GetOrgAndTeam(t, database)
	w1 := dbtest.CreateWebhookForOrg(t, database, org, webhooks.GithubPushIntegrationType)
	w2 := dbtest.CreateWebhookForOrg(t, database, org, webhooks.GithubPushIntegrationType)

	w := httptest.NewRecorder()
	r := requestAs(t, user, "DELETE", "/api/users/org/"+org.ExternalID+"/webhooks/"+w1.SecretID, nil)
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusNoContent, w.Code)

	webhooks, err := database.ListOrganizationWebhooks(context.Background(), org.ExternalID)
	assert.NoError(t, err)
	assert.Len(t, webhooks, 1)
	assert.Equal(t, w2.ID, webhooks[0].ID)
}
