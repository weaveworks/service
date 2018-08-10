package grpc

import (
	"net/http"

	"github.com/weaveworks/common/httpgrpc"
	"github.com/weaveworks/service/users"
	"golang.org/x/net/context"
)

/** LEGACY remove these after deployment **/
// LookupOrganizationWebhookUsingSecretID gets the webhook given the external org ID and the secret ID of the webhook.
func (a *usersServer) LookupOrganizationWebhookUsingSecretID(ctx context.Context, req *users.LookupOrganizationWebhookUsingSecretIDRequest) (*users.LookupOrganizationWebhookUsingSecretIDResponse, error) {
	return a.AuthWebhookSecretForOrg(ctx, req)
}

/** END LEGACY **/

// LookupOrganizationWebhookUsingSecretID gets the webhook given the external org ID and the secret ID of the webhook.
func (a *usersServer) SetOrganizationWebhookFirstSeenAt(ctx context.Context, req *users.SetOrganizationWebhookFirstSeenAtRequest) (*users.SetOrganizationWebhookFirstSeenAtResponse, error) {
	firstSeenAt, err := a.db.SetOrganizationWebhookFirstSeenAt(ctx, req.SecretID)
	if err != nil {
		return nil, err
	}
	return &users.SetOrganizationWebhookFirstSeenAtResponse{
		FirstSeenAt: firstSeenAt,
	}, nil
}
