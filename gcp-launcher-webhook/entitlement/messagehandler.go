package entitlement

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/weaveworks/service/common/gcp/procurement"
	"github.com/weaveworks/service/common/gcp/pubsub/dto"
	common_grpc "github.com/weaveworks/service/common/grpc"
	"github.com/weaveworks/service/common/orgs"
	"github.com/weaveworks/service/gcp-launcher-webhook/event"
	"github.com/weaveworks/service/users"
)

// MessageHandler handles a PubSub message.
type MessageHandler struct {
	Procurement procurement.API
	Users       users.UsersClient
}

// Handle proceeds entitlement messages from PubSub.
func (m MessageHandler) Handle(msg dto.Message) error {
	ctx := context.Background()

	var payload event.Payload
	if err := json.Unmarshal(msg.Data, &payload); err != nil {
		return err
	}

	if payload.Entitlement.ID == "" {
		// Account updates are ignored since we handle them during the signup process
		return nil // ACK
	}

	// Fetch entitlement to get to know the external account ID
	entitlementName := "entitlements/" + payload.Entitlement.ID
	ent, err := m.Procurement.GetEntitlement(ctx, entitlementName)
	switch {
	case err != nil:
		return errors.Wrapf(err, "error getting entitlement: %q", entitlementName)
	case ent == nil:
		return nil // ACK: entitlement no longer exists
	}

	externalAccountID := ent.AccountID()
	logger := log.WithFields(log.Fields{"external_account_id": externalAccountID, "entitlement": entitlementName})

	resp, err := m.Users.GetGCP(ctx, &users.GetGCPRequest{ExternalAccountID: externalAccountID})
	if err != nil {
		// If the account does not yet exist, this means the user hasn't gone through the signup.
		// It is safe to ACK this as during the signup we fetch and approve the latest entitlement,
		// as well as sync it to our database.
		if common_grpc.IsGRPCStatusErrorCode(err, http.StatusNotFound) {
			logger.Infof("Account %v has not yet finished signing up, ignoring message", externalAccountID)
			return nil // ACK
		}

		return errors.Wrapf(err, "failed getting account: %v", externalAccountID) // NACK
	}
	gcp := resp.GCP

	// Activation.
	if !gcp.Activated {
		logger.Infof("Account %v has not yet been activated, ignoring message", externalAccountID)
		return nil // ACK
	}

	if err != nil {
		// Once in a while, Google seems to be sending a PubSub message for an entitlement that is
		// no longer accessible for us. This could be due to the (billing) account being deleted on
		// Google's end.
		// If that entitlement is marked as cancelled locally, we just ignore that error to have
		// the PubSub message properly ACKed.
		// TODO(rndstr): can we confirm the account was deleted and delete the instance instead?
		if gcp.SubscriptionStatus == string(procurement.Cancelled) {
			return nil // ACK
		}
		return err
	}

	switch payload.EventType {
	case event.CreationRequested:
		if ent.State == procurement.ActivationRequested {
			// Approve the entitlement and wait for another message for when
			// it becomes active before setting up the service for the
			// customer and updating our records.
			logger.Infof("Approving entitlement: %+v", ent)
			if err := m.Procurement.ApproveEntitlement(ctx, ent.Name, ""); err != nil {
				log.Errorf("Partner failed to approve entitlement for '%s': %v", ent.AccountID(), err)
				return err
			}
			return nil
		}
		logger.Warnf("Entitlement state not %q: %+v", procurement.ActivationRequested, ent)

	case event.Active:
		if ent.State == procurement.Active {
			// Write to database after approval went through
			logger.Infof("Activating entitlement: %+v", ent)
			return m.updateEntitlement(ctx, ent)
		}
		logger.Warnf("Entitlement state not %q: %+v", procurement.Active, ent)

	case event.PlanChangeRequested:
		if ent.State == procurement.PendingPlanChangeApproval {
			// Don't write anything to our database until the entitlement
			// becomes active within the Procurement Service.
			// TODO(rndstr): is ent.NewPendingPlan the correct to send here, or do we need to extract from payload?
			if err := m.Procurement.ApprovePlanChangeEntitlement(ctx, ent.Name, ent.NewPendingPlan); err != nil {
				log.Errorf("Partner failed to approve entitlement for '%s': %v", ent.AccountID(), err)
				return err
			}
			return nil
		}
		logger.Warnf("Entitlement state not %q: %+v", procurement.PendingPlanChangeApproval, ent)

	case event.PlanChanged:
		if ent.State == procurement.Active {
			// Write to database after plan change approval went through
			logger.Infof("Updating entitlement: %+v", ent)
			return m.updateEntitlement(ctx, ent)
		}
		logger.Warnf("Entitlement state not %q: %+v", procurement.Active, ent)

	case event.PlanChangeCancelled:
		// Do nothing, we didn't save it in database yet.
		return nil

	case event.Cancelled:
		if ent.State == procurement.Cancelled {
			// Write to database
			logger.Infof("Cancelling entitlement: %+v", ent)
			return m.cancelEntitlement(ctx, ent)
		}
		return nil

	case event.PendingCancellation:
		// Do nothing. We want to cancel once it's truly canceled. For now
		// it's just set to not renew at the end of the billing cycle.
		return nil

	case event.CancellationReverted:
		// Do nothing. The service was already active, but now it's set to
		// renew automatically at the end of the billing cycle.
		return nil

	case event.Deleted:
		// Do nothing. The entitlement has to be canceled to be deleted, so
		// this has already been handled by a cancellation message.
		return nil
	}

	log.Warnf("Did not process entitlement update: %+v\n", ent)
	return nil // ACK unknown messages
}

// updateEntitlement updates an entitlement in the database.
// It should not be called when cancelling, use cancelEntitlement()
// then.
func (m MessageHandler) updateEntitlement(ctx context.Context, ent *procurement.Entitlement) error {
	accID := ent.AccountID()
	if err := m.updateGCP(ctx, ent); err != nil {
		log.Errorf("failed to update GCP for '%s': %v", accID, err)
		return err
	}

	if err := m.enableWeaveCloudAccess(ctx, accID); err != nil {
		log.Errorf("Failed to enable Weave Cloud Access for '%s': %v", accID, err)
		return err
	}
	return nil
}

// cancelEntitlement updates the entitlement status and disables access for the organization.
func (m MessageHandler) cancelEntitlement(ctx context.Context, ent *procurement.Entitlement) error {
	if err := m.disableWeaveCloudAccess(ctx, ent.AccountID()); err != nil {
		return err
	}
	return m.updateGCP(ctx, ent)
}

func (m MessageHandler) updateGCP(ctx context.Context, ent *procurement.Entitlement) error {
	_, err := m.Users.UpdateGCP(ctx, &users.UpdateGCPRequest{
		GCP: &users.GoogleCloudPlatform{
			ExternalAccountID:  ent.AccountID(),
			ConsumerID:         ent.UsageReportingID,
			SubscriptionName:   ent.Name,
			SubscriptionLevel:  ent.Plan,
			SubscriptionStatus: string(ent.State),
		},
	})
	return err
}

func (m MessageHandler) enableWeaveCloudAccess(ctx context.Context, externalAccountID string) error {
	return m.setWeaveCloudAccessFlagsTo(ctx, externalAccountID, false)
}

func (m MessageHandler) disableWeaveCloudAccess(ctx context.Context, externalAccountID string) error {
	return m.setWeaveCloudAccessFlagsTo(ctx, externalAccountID, true)
}

func (m MessageHandler) setWeaveCloudAccessFlagsTo(ctx context.Context, externalAccountID string, value bool) error {
	org, err := m.Users.GetOrganization(ctx, &users.GetOrganizationRequest{
		ID: &users.GetOrganizationRequest_GCPExternalAccountID{GCPExternalAccountID: externalAccountID},
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
