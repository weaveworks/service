package subscription_test

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/weaveworks/service/common/gcp/partner"
	"github.com/weaveworks/service/common/gcp/pubsub/dto"
	"github.com/weaveworks/service/gcp-launcher-webhook/subscription"
	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/mock_users"
	"github.com/weaveworks/service/common/gcp/partner/mock_partner"
)

var (
	gcpInactive = users.GoogleCloudPlatform{
		AccountID: "acc123",
		Activated: false,
	}
	gcpActivated = users.GoogleCloudPlatform{
		AccountID: "acc123",
		Activated: true,
	}

	msgFoo = dto.Message{
		Attributes: map[string]string{
			"name":              "partnerSubscriptions/1",
			"externalAccountId": "acc123",
		},
	}

	orgExternalID = "optimistic-organization-42"
	org           = users.Organization{
		ExternalID: orgExternalID,
	}
)

func TestMessageHandler_Handle_notFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	client := mock_users.NewMockUsersClient(ctrl)
	client.EXPECT().
		GetGCP(ctx, &users.GetGCPRequest{AccountID: "acc123"}).
		Return(nil, errors.New("boom"))
	p := mock_partner.NewMockAPI(ctrl)

	mh := subscription.MessageHandler{Users: client, Partner: p}
	err := mh.Handle(msgFoo)
	assert.Error(t, err)
}

func TestMessageHandler_Handle_inactive(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	client := mock_users.NewMockUsersClient(ctrl)
	client.EXPECT().
		GetGCP(ctx, &users.GetGCPRequest{AccountID: "acc123"}).
		Return(&users.GetGCPResponse{GCP: gcpInactive}, nil)
	p := mock_partner.NewMockAPI(ctrl)

	mh := subscription.MessageHandler{Users: client, Partner: p}
	err := mh.Handle(msgFoo)
	assert.NoError(t, err)
}

func TestMessageHandler_Handle_cancel(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	client := mock_users.NewMockUsersClient(ctrl)
	client.EXPECT().
		GetGCP(ctx, &users.GetGCPRequest{AccountID: "acc123"}).
		Return(&users.GetGCPResponse{GCP: gcpActivated}, nil)
	client.EXPECT().
		GetOrganization(ctx, &users.GetOrganizationRequest{ID: &users.GetOrganizationRequest_GCPAccountID{GCPAccountID: "acc123"}}).
		Return(&users.GetOrganizationResponse{Organization: org}, nil)
	client.EXPECT().
		SetOrganizationFlag(ctx, &users.SetOrganizationFlagRequest{ExternalID: orgExternalID, Flag: "RefuseDataAccess", Value: true}).
		Return(&users.SetOrganizationFlagResponse{}, nil)
	client.EXPECT().
		SetOrganizationFlag(ctx, &users.SetOrganizationFlagRequest{ExternalID: orgExternalID, Flag: "RefuseDataUpload", Value: true}).
		Return(&users.SetOrganizationFlagResponse{}, nil)
	client.EXPECT().
		UpdateGCP(ctx, &users.UpdateGCPRequest{
			GCP: &users.GoogleCloudPlatform{
				AccountID:         "acc123",
				Activated:         false,
				ConsumerID:        "",
				SubscriptionName:  "",
				SubscriptionLevel: "",
			}}).
		Return(nil, nil)

	p := mock_partner.NewMockAPI(ctrl)
	p.EXPECT().
		ListSubscriptions(gomock.Any(), "acc123").
		Return([]partner.Subscription{
			{ // this one has been canceled
				Name:              "partnerSubscriptions/1",
				ExternalAccountID: "acc123",
				Status:            partner.Complete,
			},
			{ // a previously canceled subscription
				Name:              "partnerSubscriptions/99",
				ExternalAccountID: "acc123",
				Status:            partner.Complete,
			},
		}, nil)

	mh := subscription.MessageHandler{Users: client, Partner: p}
	err := mh.Handle(msgFoo)
	assert.NoError(t, err)
}

func TestMessageHandler_Handle_reactivationPlanChange(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	client := mock_users.NewMockUsersClient(ctrl)
	client.EXPECT().
		GetGCP(ctx, &users.GetGCPRequest{AccountID: "acc123"}).
		Return(&users.GetGCPResponse{GCP: gcpActivated}, nil)
	client.EXPECT().
		GetOrganization(ctx, &users.GetOrganizationRequest{ID: &users.GetOrganizationRequest_GCPAccountID{GCPAccountID: "acc123"}}).
		Return(&users.GetOrganizationResponse{Organization: org}, nil)
	client.EXPECT().
		SetOrganizationFlag(ctx, &users.SetOrganizationFlagRequest{ExternalID: orgExternalID, Flag: "RefuseDataAccess", Value: false}).
		Return(&users.SetOrganizationFlagResponse{}, nil)
	client.EXPECT().
		SetOrganizationFlag(ctx, &users.SetOrganizationFlagRequest{ExternalID: orgExternalID, Flag: "RefuseDataUpload", Value: false}).
		Return(&users.SetOrganizationFlagResponse{}, nil)
	client.EXPECT().
		UpdateGCP(ctx, &users.UpdateGCPRequest{
			GCP: &users.GoogleCloudPlatform{
				AccountID:         "acc123",
				ConsumerID:        "project_number:123",
				SubscriptionName:  "partnerSubscriptions/1",
				SubscriptionLevel: "enterprise",
			}}).
		Return(nil, nil)

	p := mock_partner.NewMockAPI(ctrl)
	p.EXPECT().
		ListSubscriptions(gomock.Any(), "acc123").
		Return([]partner.Subscription{
			{
				Name:              "partnerSubscriptions/1",
				ExternalAccountID: "acc123",
				Status:            partner.Pending,
				SubscribedResources: []partner.SubscribedResource{
					{
						SubscriptionProvider: "weaveworks-public-cloudmarketplacepartner.googleapis.com",
						Resource:             "weave-cloud",
						Labels: map[string]string{
							"weaveworks-public-cloudmarketplacepartner.googleapis.com/ServiceLevel": "enterprise",
							"consumerId": "project_number:123",
						},
					},
				},
			},
			{
				Name:              "partnerSubscriptions/99",
				ExternalAccountID: "acc123",
				Status:            partner.Complete,
			},
		}, nil)
	expectedBody := &partner.RequestBody{
		ApprovalID: "default-approval",
		Labels: map[string]string{"keyForSSOLogin": "acc123"},
	}
	p.EXPECT().
		ApproveSubscription(gomock.Any(), "partnerSubscriptions/1", expectedBody)


	mh := subscription.MessageHandler{Users: client, Partner: p}
	err := mh.Handle(msgFoo)
	assert.NoError(t, err)
}
