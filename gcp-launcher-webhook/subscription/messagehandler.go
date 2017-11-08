package subscription

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"

	"github.com/weaveworks/service/common/gcp/partner"
	"github.com/weaveworks/service/common/gcp/pubsub/dto"
	"github.com/weaveworks/service/users"
)

const (
	ssoLoginKey          = "keyForSsoLogin"
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
	gcpAccountID := msg.Attributes[externalAccountIDKey]
	subscriptionName := msg.Attributes[subscriptionNameKey]

	// Fetch respective organization
	resp, err := m.Users.GetOrganization(context.Background(), &users.GetOrganizationRequest{
		ID: &users.GetOrganizationRequest_GCPAccountID{GCPAccountID: gcpAccountID},
	})
	if err != nil {
		return fmt.Errorf("cannot find organization with GCP account ID: %v", gcpAccountID)
	}
	org := resp.Organization

	// Activation.
	if !org.GCP.Active {
		log.Infof("GCP account has not yet been activated, ignoring message for org: %v", org.ExternalID)
		return nil // ACK
	}

	sub, subs, err := m.getSubscriptions(gcpAccountID, subscriptionName)
	if err != nil {
		return err
	}

	// Cancel.
	if sub.Status == partner.StatusComplete {
		hasPending := false
		for _, s := range subs {
			if s.Status == partner.StatusPending {
				hasPending = true
				break
			}
		}
		if !hasPending {
			return m.cancelSubscription(org, sub)
		}
	}

	// Reactivation, PlanChange.
	//
	// This covers both
	// - reactivation after cancellation: no other active subscription
	// - changing of plan: has other active subscription
	if sub.Status == partner.StatusPending {
		return m.updateSubscription(org, sub)
	}

	if sub.Status == partner.StatusActive {
		// Subscriptions are activated by first going through the pending state.
		// The pending status has already been processed or if the account is
		// freshly created, we will update the subscription through that flow.
		return nil
	}

	log.Warnf("Did not process subscription update for org %v: %+v\nAll: %+v", org.ExternalID, *sub, subs)
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
func (m MessageHandler) updateSubscription(org users.Organization, sub *partner.Subscription) error {
	ctx := context.Background()

	// Set organization subscription
	level := sub.ExtractLabel("weave-cloud", "ServiceLevel")
	_ = level
	// FIXME(rndstr): the ConsumerID key is most definitely wrong
	consumerID := sub.ExtractLabel("weave-cloud", "ConsumerID")
	_ = consumerID

	// FIXME(rndstr): m.Users.SetOrganizationGCP(consumerID, approved.Name, level)

	// Approve subscription
	body := &partner.RequestBody{
		ApprovalID: "default-approval",
		Labels: map[string]string{
			// The value passed here will be sent to us during SSO. It allows us to
			// verify who the user is and log him in.
			ssoLoginKey: org.GCP.AccountID,
		},
	}

	// TODO(rndstr): retry logic here
	if _, err := m.Partner.ApproveSubscription(ctx, sub.Name, body); err != nil {
		return err
	}

	return nil
}

// cancelSubscriptions removes the subscription from the organization.
func (m MessageHandler) cancelSubscription(org users.Organization, sub *partner.Subscription) error {
	// FIXME(rndstr): m.Users.SetOrganizationGCP("", "", "")
	return nil
}

// getSubscriptions fetches all subscriptions of the account. Furthermore, it picks the subscription with the
// given subscriptionName.
func (m MessageHandler) getSubscriptions(gcpAccountID string, subscriptionName string) (*partner.Subscription, []partner.Subscription, error) {
	subs, err := m.Partner.ListSubscriptions(context.Background(), gcpAccountID)
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
