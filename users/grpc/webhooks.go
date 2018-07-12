package grpc

import (
	"net/http"

	"github.com/weaveworks/common/httpgrpc"
	"github.com/weaveworks/service/users"
	"golang.org/x/net/context"
)

// LookupOrganizationWebhookUsingSecretID gets the webhook given the external org ID and the secret ID of the webhook.
func (a *usersServer) LookupOrganizationWebhookUsingSecretID(ctx context.Context, req *users.LookupOrganizationWebhookUsingSecretIDRequest) (*users.LookupOrganizationWebhookUsingSecretIDResponse, error) {
	webhook, err := a.db.FindOrganizationWebhookBySecretID(ctx, req.SecretID)
	if err == users.ErrNotFound {
		err = httpgrpc.Errorf(http.StatusNotFound, "Webhook does not exist.")
	}
	if err != nil {
		return nil, err
	}
	return &users.LookupOrganizationWebhookUsingSecretIDResponse{
		Webhook: webhook,
	}, nil
}

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
