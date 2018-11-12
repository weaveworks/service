package grpc

import (
	"net/http"

	"github.com/weaveworks/common/httpgrpc"
	"github.com/weaveworks/service/users"
	"golang.org/x/net/context"
)

// LookupOrganizationWebhookUsingSecretID gets the webhook given the
// secret ID of the webhook. It also checks that the organization to
// which that web hook belongs has 'data upload' enabled, i.e. hasn't
// been cut off because of lack of payment.
func (a *usersServer) LookupOrganizationWebhookUsingSecretID(ctx context.Context, req *users.LookupOrganizationWebhookUsingSecretIDRequest) (*users.LookupOrganizationWebhookUsingSecretIDResponse, error) {
	webhook, err := a.db.FindOrganizationWebhookBySecretID(ctx, req.SecretID)
	if err == users.ErrNotFound {
		err = httpgrpc.Errorf(http.StatusNotFound, "Webhook does not exist.")
	}
	if err != nil {
		return nil, err
	}
	org, err := a.db.FindOrganizationByInternalID(ctx, webhook.OrganizationID)
	if err != nil {
		return nil, err
	}
	// For now treat all webhook invocations as 'data uploads'; we
	// may want to be more discriminating at some point.
	if err := authorizeAction(users.INSTANCE_DATA_UPLOAD, org); err != nil {
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
