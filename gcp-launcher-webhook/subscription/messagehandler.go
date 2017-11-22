package subscription

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/weaveworks/service/common/gcp/partner"
	"github.com/weaveworks/service/common/gcp/pubsub/dto"
	"github.com/weaveworks/service/common/orgs"
	"github.com/weaveworks/service/users"
)

const (
	externalAccountIDKey = "externalAccountId"
	subscriptionNameKey  = "name"
)

// MessageHandler handles a PubSub message.
type MessageHandler struct {
	Partner partner.API
	Users   users.UsersClient
}

// Handle processes the message for a subscription. It fetches organization and the
func (m MessageHandler) Handle(msg dto.Message) error {
	ctx := context.Background()
	gcpAccountID := msg.Attributes[externalAccountIDKey]
	subscriptionName := msg.Attributes[subscriptionNameKey]
	logger := log.WithFields(log.Fields{"account_id": gcpAccountID, "subscription": subscriptionName})

	resp, err := m.Users.GetGCP(ctx, &users.GetGCPRequest{AccountID: gcpAccountID})
	if err != nil {
		return errors.Wrapf(err, "cannot find account: %v", gcpAccountID) // NACK
	}
	gcp := resp.GCP

	// Activation.
	if !gcp.Activated {
		logger.Infof("Account %v has not yet been activated, ignoring message", gcpAccountID)
		return nil // ACK
	}

	sub, subs, err := m.getSubscriptions(ctx, gcpAccountID, subscriptionName)
	if err != nil {
		return err
	}

	// Cancel.
	if sub.Status == partner.Complete {
		if hasOtherSubscription(partner.Pending, subs) || hasOtherSubscription(partner.Active, subs) {
			logger.Info("Not cancelling subscription because there is another one either pending or active: %+v", sub)
			return nil // ACK
		}
		logger.Infof("Cancelling subscription: %+v", sub)
		return m.cancelSubscription(ctx, sub)
	}

	// Reactivation, PlanChange.
	//
	// This covers both
	// - reactivation after cancellation: no other active subscription
	// - changing of plan: has other active subscription
	if sub.Status == partner.Pending {
		logger.Infof("Activating subscription: %+v", sub)
		return m.updateSubscription(ctx, sub)
	}

	if sub.Status == partner.Active {
		logger.Infof("No action for active subscription: %+v", sub)
		// Subscriptions are activated by first going through the pending state.
		// The pending status has already been processed or if the account is
		// freshly created, we will update the subscription through that flow.
		return nil
	}

	log.Warnf("Did not process subscription update: %+v\nAll: %+v", *sub, subs)
	return nil // ACK unknown messages
}

// updateSubscription handles the event of changing the subscription plan. It may either
// be changing from subscription X to Y or reinstating a previously cancelled subscription.
//
// pre:
//   A: old subscription --> ACTIVE, new subscription --> PENDING
//   B: old subscription --> !ACTIVE, new subscription --> PENDING
// run: user.set(subscription)
// run: partner.approve(subscription) - send account id as label for SSO
//       retry on failure, otherwise subscriptions might get out of sync
// return: ack.
// post: new subscription --> ACTIVE
func (m MessageHandler) updateSubscription(ctx context.Context, sub *partner.Subscription) error {
	level := sub.ExtractResourceLabel("weave-cloud", partner.ServiceLevelLabelKey)
	consumerID := sub.ExtractResourceLabel("weave-cloud", partner.ConsumerIDLabelKey)

	_, err := m.Users.UpdateGCP(ctx, &users.UpdateGCPRequest{
		GCP: &users.GoogleCloudPlatform{
			AccountID:         sub.ExternalAccountID,
			ConsumerID:        consumerID,
			SubscriptionName:  sub.Name,
			SubscriptionLevel: level,
		},
	})
	if err != nil {
		return err
	}

	if err := m.enableWeaveCloudAccess(ctx, sub.ExternalAccountID); err != nil {
		return err
	}

	// Approve subscription
	body := partner.RequestBodyWithSSOLoginKey(sub.ExternalAccountID)
	if _, err := m.Partner.ApproveSubscription(ctx, sub.Name, body); err != nil {
		return err
	}

	return nil
}

// cancelSubscriptions removes the subscription from the organization.
func (m MessageHandler) cancelSubscription(ctx context.Context, sub *partner.Subscription) error {
	if err := m.disableWeaveCloudAccess(ctx, sub.ExternalAccountID); err != nil {
		return err
	}

	// The account ID is kept intact to detect a customer reactivating their subscription.
	_, err := m.Users.UpdateGCP(ctx, &users.UpdateGCPRequest{
		GCP: &users.GoogleCloudPlatform{
			AccountID:         sub.ExternalAccountID,
			ConsumerID:        "",
			SubscriptionName:  "",
			SubscriptionLevel: "",
		},
	})
	return err
}

func (m MessageHandler) enableWeaveCloudAccess(ctx context.Context, gcpAccountID string) error {
	return m.setWeaveCloudAccessFlagsTo(ctx, gcpAccountID, false)
}

func (m MessageHandler) disableWeaveCloudAccess(ctx context.Context, gcpAccountID string) error {
	return m.setWeaveCloudAccessFlagsTo(ctx, gcpAccountID, true)
}

func (m MessageHandler) setWeaveCloudAccessFlagsTo(ctx context.Context, gcpAccountID string, value bool) error {
	org, err := m.Users.GetOrganization(ctx, &users.GetOrganizationRequest{
		ID: &users.GetOrganizationRequest_GCPAccountID{GCPAccountID: gcpAccountID},
	})
	if err != nil {
		return err
	}
	if _, err := m.Users.SetOrganizationFlag(ctx, &users.SetOrganizationFlagRequest{
		ExternalID: org.Organization.ExternalID, Flag: orgs.RefuseDataAccess, Value: value}); err != nil {
		return err
	}
	if _, err := m.Users.SetOrganizationFlag(ctx, &users.SetOrganizationFlagRequest{
		ExternalID: org.Organization.ExternalID, Flag: orgs.RefuseDataUpload, Value: value}); err != nil {
		return err
	}
	return nil
}

// getSubscriptions fetches all subscriptions of the account. Furthermore, it picks the subscription with the
// given subscriptionName.
func (m MessageHandler) getSubscriptions(ctx context.Context, gcpAccountID string, subscriptionName string) (*partner.Subscription, []partner.Subscription, error) {
	subs, err := m.Partner.ListSubscriptions(ctx, gcpAccountID)
	if err != nil {
		return nil, nil, err
	}

	var sub *partner.Subscription
	for _, s := range subs {
		if s.Name == subscriptionName {
			sub = &s
			break
		}
	}
	if sub == nil {
		return nil, nil, fmt.Errorf("referenced subscription not found: %v", subscriptionName)
	}

	return sub, subs, nil
}

// hasOtherSubscription return true if there is a subscription among the provided ones with status equal to the provided one, or false otherwise.
func hasOtherSubscription(status partner.SubscriptionStatus, subs []partner.Subscription) bool {
	for _, s := range subs {
		if s.Status == status {
			return true
		}
	}
	return false
}
