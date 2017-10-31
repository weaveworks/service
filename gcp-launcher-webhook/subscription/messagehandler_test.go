package subscription_test

import (
	"context"
	"github.com/weaveworks/service/common/gcp/partner"
	"github.com/golang/mock/gomock"
	"github.com/weaveworks/service/users/mock_users"
	"github.com/weaveworks/service/gcp-launcher-webhook/subscription"
	"testing"
	"github.com/weaveworks/service/users"
)

type partnerMock struct {
	subcriptions []partner.Subscription
}

func (m partnerMock) ApproveSubscription(ctx context.Context, name string, body *partner.RequestBody) (*partner.Subscription, error) {
	return nil, nil
}
func (m partnerMock) DenySubscription(ctx context.Context, name string, body *partner.RequestBody) (*partner.Subscription, error) {
	return nil, nil
}
func (m partnerMock) GetSubscription(ctx context.Context, name string) (*partner.Subscription, error) {
	return nil, nil
}
func (m partnerMock) ListSubscriptions(ctx context.Context, externalAccountID string) ([]partner.Subscription, error) {
	return m.subcriptions, nil
}

func TestMessageHandler_Handle(t *testing.T) {
	t.SkipNow()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	client := mock_users.NewMockUsersClient(ctrl)
	client.EXPECT().
		GetOrganization(ctx, &users.GetOrganizationRequest{
			ID: &users.GetOrganizationRequest_ExternalID{"foo"},
		}).
		Return(&users.GetOrganizationResponse{
			Organization: users.Organization{
				ExternalID: "foo",
			},
		})

	mh := subscription.MessageHandler{Users: client, Partner: &partnerMock{}}
	_ = mh
}
