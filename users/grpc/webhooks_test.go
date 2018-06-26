package grpc_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/db/dbtest"
)

func Test_LookupOrganizationWebhookUsingSecretID(t *testing.T) {
	setup(t)
	defer cleanup(t)

	_, org := dbtest.GetOrg(t, database)
	webhook := dbtest.CreateWebhookForOrg(t, database, org, "github")

	response, err := server.LookupOrganizationWebhookUsingSecretID(ctx, &users.LookupOrganizationWebhookUsingSecretIDRequest{
		OrgExternalID: org.ExternalID,
		SecretID:      webhook.SecretID,
	})
	assert.NoError(t, err)
	assert.Equal(t, webhook.ID, response.Webhook.ID)
}
