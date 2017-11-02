package subscription_test

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"

	"errors"
	"github.com/stretchr/testify/assert"
	"github.com/weaveworks/service/common/gcp/partner"
	"github.com/weaveworks/service/common/gcp/pubsub/dto"
	"github.com/weaveworks/service/gcp-launcher-webhook/subscription"
	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/mock_users"
)

type partnerMock struct {
	subscriptions []partner.Subscription
	approved      bool
	denied        bool
	body          *partner.RequestBody
}

func (m *partnerMock) ApproveSubscription(ctx context.Context, name string, body *partner.RequestBody) (*partner.Subscription, error) {
	m.approved = true
	m.body = body
	return nil, nil
}
func (m *partnerMock) DenySubscription(ctx context.Context, name string, body *partner.RequestBody) (*partner.Subscription, error) {
	m.denied = true
	m.body = body
	return nil, nil
}
func (m *partnerMock) GetSubscription(ctx context.Context, name string) (*partner.Subscription, error) {
	return nil, nil
}
func (m *partnerMock) ListSubscriptions(ctx context.Context, externalAccountID string) ([]partner.Subscription, error) {
	return m.subscriptions, nil
}

var (
	orgInactive = users.Organization{
		ExternalID: "is-inactive-1",
		GCP: &users.GoogleCloudPlatform{
			Active: false,
		},
	}
	orgActive = users.Organization{
		ExternalID: "is-active-2",
		GCP: &users.GoogleCloudPlatform{
			Active:    true,
			AccountID: "gcpacc123",
		},
	}

	msgFoo = dto.Message{
		Attributes: map[string]string{
			"name":              "partnerSubscriptions/1",
			"externalAccountId": "foo",
		},
	}
)

func TestMessageHandler_Handle_inactive(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	client := mock_users.NewMockUsersClient(ctrl)
	client.EXPECT().
		GetOrganization(ctx, &users.GetOrganizationRequest{
			ID: &users.GetOrganizationRequest_GCPAccountID{GCPAccountID: "foo"},
		}).
		Return(&users.GetOrganizationResponse{Organization: orgInactive}, nil)

	mh := subscription.MessageHandler{Users: client, Partner: &partnerMock{}}
	err := mh.Handle(msgFoo)
	assert.NoError(t, err)
}

func TestMessageHandler_Handle_notFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	client := mock_users.NewMockUsersClient(ctrl)
	client.EXPECT().
		GetOrganization(ctx, &users.GetOrganizationRequest{
			ID: &users.GetOrganizationRequest_GCPAccountID{GCPAccountID: "foo"},
		}).
		Return(nil, errors.New("boom"))

	mh := subscription.MessageHandler{Users: client, Partner: &partnerMock{}}
	err := mh.Handle(msgFoo)
	assert.Error(t, err)
}

func TestMessageHandler_Handle_cancel(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	client := mock_users.NewMockUsersClient(ctrl)
	client.EXPECT().
		GetOrganization(ctx, &users.GetOrganizationRequest{
			ID: &users.GetOrganizationRequest_GCPAccountID{GCPAccountID: "foo"},
		}).
		Return(&users.GetOrganizationResponse{Organization: orgActive}, nil)

	/*
		client.EXPECT().
			SetOrganizationGCP("", "", "")
			Return(nil)
	*/

	p := &partnerMock{
		subscriptions: []partner.Subscription{
			{
				Name:   "partnerSubscriptions/1",
				Status: partner.StatusComplete,
			},
			{
				Name:   "partnerSubscriptions/99",
				Status: partner.StatusComplete,
			},
		},
	}
	mh := subscription.MessageHandler{Users: client, Partner: p}
	err := mh.Handle(msgFoo)
	assert.NoError(t, err)
	assert.False(t, p.approved)
	assert.False(t, p.denied)
}

func TestMessageHandler_Handle_reactivationPlanChange(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	client := mock_users.NewMockUsersClient(ctrl)
	client.EXPECT().
		GetOrganization(ctx, &users.GetOrganizationRequest{
			ID: &users.GetOrganizationRequest_GCPAccountID{GCPAccountID: "foo"},
		}).
		Return(&users.GetOrganizationResponse{Organization: orgActive}, nil)

	/*
		client.EXPECT().
			SetOrganizationGCP("partnerSubscriptions/1", "standard", "…")
			Return(nil)
	*/

	p := &partnerMock{
		subscriptions: []partner.Subscription{
			{
				Name:   "partnerSubscriptions/1",
				Status: partner.StatusPending,
				SubscribedResources: []partner.SubscribedResource{
					{
						SubscriptionProvider: "weaveworks-public-cloudmarketplacepartner.googleapis.com",
						Resource:             "weave-cloud",
						Labels: map[string]string{
							"weaveworks-public-cloudmarketplacepartner.googleapis.com/ServiceLevel": "standard",
						},
					},
				},
			},
			{
				Name:   "partnerSubscriptions/99",
				Status: partner.StatusComplete,
			},
		},
	}
	mh := subscription.MessageHandler{Users: client, Partner: p}
	err := mh.Handle(msgFoo)
	assert.NoError(t, err)
	assert.True(t, p.approved)
	assert.Equal(t, "gcpacc123", p.body.Labels["keyForSsoLogin"])
	assert.False(t, p.denied)
}
