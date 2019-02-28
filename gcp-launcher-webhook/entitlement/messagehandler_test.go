package entitlement_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/status"

	"github.com/weaveworks/service/common/gcp/procurement"
	"github.com/weaveworks/service/common/gcp/procurement/mock_procurement"
	"github.com/weaveworks/service/common/gcp/pubsub/dto"
	"github.com/weaveworks/service/gcp-launcher-webhook/entitlement"
	"github.com/weaveworks/service/gcp-launcher-webhook/event"
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
	entID               = "1"
	entName             = "providers/weaveworks-dev/entitlements/1"
	entMsg              = makeMessage(event.CreationRequested, entID, "")
	accMsg              = makeMessage("SOMETHING", "", externalAccountID)
	entUsageReportingID = "product_number:123"
	entPlan             = "standard"
	entNewPlan          = "enterprise"
	ent                 = makeEntitlement(entID, procurement.ActivationRequested)
	entNew              = func() *procurement.Entitlement {
		e := makeEntitlement(entID, procurement.Active)
		e.Plan = entNewPlan
		return e
	}()

	orgExternalID = "optimistic-organization-42"
	org           = users.Organization{
		ExternalID: orgExternalID,
	}
)

func makeMessage(eventType event.Type, entitlementID, accountID string) dto.Message {
	p := event.Payload{
		EventID:   fmt.Sprintf("eventid-%d", rand.Int63()),
		EventType: eventType,
	}
	if entitlementID != "" {
		p.Entitlement.ID = entitlementID
	}
	if accountID != "" {
		p.Account.ID = accountID
	}
	bs, _ := json.Marshal(p)
	return dto.Message{
		Data:      bs,
		MessageID: fmt.Sprintf("msgid-%d", rand.Int63()),
	}
}

func TestMessageHandler_Handle_ignoreAccountMessage(t *testing.T) {
	mh := entitlement.MessageHandler{}
	err := mh.Handle(accMsg)
	assert.NoError(t, err)
}

// TestMessageHandler_Handle_notFound verifies that an Account «Not found» error
// does not lead to an error because we expect the account not to be found before
// signup has finished.
func TestMessageHandler_Handle_notFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.TODO()
	p := mock_procurement.NewMockAPI(ctrl)
	expectGetEntitlement(p, ent)
	client := mock_users.NewMockUsersClient(ctrl)
	client.EXPECT().
		GetGCP(ctx, &users.GetGCPRequest{ExternalAccountID: externalAccountID}).
		Return(nil, status.Error(404, "not found"))

	mh := entitlement.MessageHandler{Users: client, Procurement: p}
	err := mh.Handle(entMsg)
	assert.NoError(t, err)
}

// TestMessageHandler_Handle_getError says that we should get an error if GetGCP()
// fails for anything other than «Not found».
func TestMessageHandler_Handle_getError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.TODO()
	p := mock_procurement.NewMockAPI(ctrl)
	expectGetEntitlement(p, ent)
	client := mock_users.NewMockUsersClient(ctrl)
	client.EXPECT().
		GetGCP(ctx, &users.GetGCPRequest{ExternalAccountID: externalAccountID}).
		Return(nil, errors.New("boom"))

	mh := entitlement.MessageHandler{Users: client, Procurement: p}
	err := mh.Handle(entMsg)
	assert.Error(t, err)
}

func TestMessageHandler_Handle_gcpInacative(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	p := mock_procurement.NewMockAPI(ctrl)
	expectGetEntitlement(p, ent)
	client := mock_users.NewMockUsersClient(ctrl)
	client.EXPECT().
		GetGCP(context.TODO(), &users.GetGCPRequest{ExternalAccountID: externalAccountID}).
		Return(&users.GetGCPResponse{GCP: gcpInactive}, nil)

	mh := entitlement.MessageHandler{Users: client, Procurement: p}
	err := mh.Handle(entMsg)
	assert.NoError(t, err)
}

func TestMessageHandler_Handle_cancelled(t *testing.T) {
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
				ExternalAccountID:  externalAccountID,
				ConsumerID:         entUsageReportingID,
				SubscriptionName:   entName,
				SubscriptionLevel:  entPlan,
				SubscriptionStatus: string(procurement.Cancelled),
			}}).
		Return(nil, nil)

	p := mock_procurement.NewMockAPI(ctrl)
	expectGetEntitlement(p, makeEntitlement(entID, procurement.Cancelled))

	mh := entitlement.MessageHandler{Users: client, Procurement: p}
	err := mh.Handle(makeMessage(event.Cancelled, entID, ""))
	assert.NoError(t, err)
}

func TestMessageHandler_Handle_changePlan(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.TODO()
	client := mock_users.NewMockUsersClient(ctrl)
	p := mock_procurement.NewMockAPI(ctrl)
	mh := entitlement.MessageHandler{Users: client, Procurement: p}

	t.Run("request approval", func(t *testing.T) {
		client.EXPECT().
			GetGCP(ctx, &users.GetGCPRequest{ExternalAccountID: externalAccountID}).
			Return(&users.GetGCPResponse{GCP: gcpActivated}, nil)
		expectGetEntitlement(p, makeEntitlement(entID, procurement.PendingPlanChangeApproval))
		p.EXPECT().
			ApprovePlanChangeEntitlement(gomock.Any(), entName, entNewPlan)

		err := mh.Handle(makeMessage(event.PlanChangeRequested, entID, ""))
		assert.NoError(t, err)
	})
	t.Run("write to database", func(t *testing.T) {
		expectGetEntitlement(p, entNew)

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
					ExternalAccountID:  externalAccountID,
					ConsumerID:         entUsageReportingID,
					SubscriptionName:   entName,
					SubscriptionLevel:  entNewPlan,
					SubscriptionStatus: string(procurement.Active),
				}}).
			Return(nil, nil)

		err := mh.Handle(makeMessage(event.PlanChanged, entID, ""))
		assert.NoError(t, err)
	})
}

func makeEntitlement(id string, state procurement.EntitlementState) *procurement.Entitlement {
	return &procurement.Entitlement{
		Name:             fmt.Sprintf("providers/weaveworks-dev/entitlements/%s", id),
		Account:          fmt.Sprintf("providers/weaveworks-dev/accounts/%s", externalAccountID),
		Provider:         "weaveworks",
		Product:          "weave-cloud",
		Plan:             entPlan,
		State:            state,
		NewPendingPlan:   entNewPlan,
		UsageReportingID: entUsageReportingID,
	}
}

func expectGetEntitlement(api *mock_procurement.MockAPI, e *procurement.Entitlement) {
	api.EXPECT().
		ResourceName("entitlements", entID).
		Return(entName)
	api.EXPECT().
		GetEntitlement(context.TODO(), entName).
		Return(e, nil)
}

