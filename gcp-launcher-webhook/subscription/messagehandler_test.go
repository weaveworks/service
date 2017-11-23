package subscription_test

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/weaveworks/service/common/gcp/partner"
	"github.com/weaveworks/service/common/gcp/partner/mock_partner"
	"github.com/weaveworks/service/common/gcp/pubsub/dto"
	"github.com/weaveworks/service/gcp-launcher-webhook/subscription"
	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/mock_users"
)

const externalAccountID = "E-F65F-C51C-67FE-D42F"

var (
	gcpInactive = users.GoogleCloudPlatform{
		ExternalAccountID: externalAccountID,
		Activated:         false,
	}
	gcpActivated = users.GoogleCloudPlatform{
		ExternalAccountID: externalAccountID,
		Activated:         true,
	}

	msgFoo = dto.Message{
		Attributes: map[string]string{
			"name":              "partnerSubscriptions/1",
			"externalAccountId": externalAccountID,
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
		GetGCP(ctx, &users.GetGCPRequest{ExternalAccountID: externalAccountID}).
		Return(nil, errors.New("boom"))
	p := mock_partner.NewMockAPI(ctrl)

	mh := subscription.MessageHandler{Users: client, Partner: p}
	err := mh.Handle(msgFoo)
	assert.NoError(t, err)
}

func TestMessageHandler_Handle_inactive(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	client := mock_users.NewMockUsersClient(ctrl)
	client.EXPECT().
		GetGCP(ctx, &users.GetGCPRequest{ExternalAccountID: externalAccountID}).
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
		GetGCP(ctx, &users.GetGCPRequest{ExternalAccountID: externalAccountID}).
		Return(&users.GetGCPResponse{GCP: gcpActivated}, nil)
	client.EXPECT().
		GetOrganization(ctx, &users.GetOrganizationRequest{
			ID: &users.GetOrganizationRequest_GCPExternalAccountID{GCPExternalAccountID: externalAccountID},
		}).
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
				ExternalAccountID: externalAccountID,
				ConsumerID:        "project_number:123",
				SubscriptionName:  "partnerSubscriptions/1",
				SubscriptionLevel: "enterprise",
			}}).
		Return(nil, nil)

	p := mock_partner.NewMockAPI(ctrl)
	p.EXPECT().
		ListSubscriptions(gomock.Any(), externalAccountID).
		Return([]partner.Subscription{
			makeSubscription("1", partner.Complete),
			makeSubscription("99", partner.Complete),
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
		GetGCP(ctx, &users.GetGCPRequest{ExternalAccountID: externalAccountID}).
		Return(&users.GetGCPResponse{GCP: gcpActivated}, nil)
	client.EXPECT().
		GetOrganization(ctx, &users.GetOrganizationRequest{
			ID: &users.GetOrganizationRequest_GCPExternalAccountID{GCPExternalAccountID: externalAccountID},
		}).
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
				ExternalAccountID: externalAccountID,
				ConsumerID:        "project_number:123",
				SubscriptionName:  "partnerSubscriptions/1",
				SubscriptionLevel: "enterprise",
			}}).
		Return(nil, nil)

	p := mock_partner.NewMockAPI(ctrl)
	p.EXPECT().
		ListSubscriptions(gomock.Any(), externalAccountID).
		Return([]partner.Subscription{
			makeSubscription("1", partner.Pending),
			makeSubscription("99", partner.Complete),
		}, nil)
	expectedBody := &partner.RequestBody{
		ApprovalID: "default-approval",
		Labels:     map[string]string{"keyForSSOLogin": externalAccountID},
	}
	p.EXPECT().
		ApproveSubscription(gomock.Any(), "partnerSubscriptions/1", expectedBody)

	mh := subscription.MessageHandler{Users: client, Partner: p}
	err := mh.Handle(msgFoo)
	assert.NoError(t, err)
}

func makeSubscription(id string, status partner.SubscriptionStatus) partner.Subscription {
	return partner.Subscription{
		Name:              "partnerSubscriptions/" + id,
		ExternalAccountID: externalAccountID,
		Status:            status,
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
	}
}
