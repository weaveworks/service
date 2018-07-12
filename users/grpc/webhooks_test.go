package grpc_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/weaveworks/service/common/constants/webhooks"
	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/db/dbtest"
)

func Test_LookupOrganizationWebhookUsingSecretID(t *testing.T) {
	setup(t)
	defer cleanup(t)

	_, org := dbtest.GetOrg(t, database)
	webhook := dbtest.CreateWebhookForOrg(t, database, org, webhooks.GithubPushIntegrationType)
	assert.Empty(t, webhook.FirstSeenAt)

	response, err := server.LookupOrganizationWebhookUsingSecretID(ctx, &users.LookupOrganizationWebhookUsingSecretIDRequest{
		SecretID: webhook.SecretID,
	})
	assert.NoError(t, err)
	assert.Equal(t, webhook.ID, response.Webhook.ID)

	webhook = dbtest.GetOrgWebhook(t, database, webhook.SecretID)
	assert.NotEmpty(t, webhook.FirstSeenAt)
}
