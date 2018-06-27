package grpc

import (
	"github.com/weaveworks/service/users"
	"golang.org/x/net/context"
)

// LookupOrganizationWebhookUsingSecretID gets the webhook given the external org ID and the secret ID of the webhook.
func (a *usersServer) LookupOrganizationWebhookUsingSecretID(ctx context.Context, req *users.LookupOrganizationWebhookUsingSecretIDRequest) (*users.LookupOrganizationWebhookUsingSecretIDResponse, error) {
	webhook, err := a.db.FindOrganizationWebhookBySecretID(ctx, req.SecretID)
	if err == users.ErrNotFound {
		err = users.ErrInvalidAuthenticationData
	}
	if err != nil {
		return nil, err
	}
	return &users.LookupOrganizationWebhookUsingSecretIDResponse{
		Webhook: webhook,
	}, nil
}
