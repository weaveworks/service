package api

import (
	"context"

	"github.com/weaveworks/common/logging"
	"github.com/weaveworks/common/user"

	"github.com/weaveworks/service/billing-api/trial"
	"github.com/weaveworks/service/common/billing/grpc"
	"github.com/weaveworks/service/common/zuora"
)

// GetBillingStatus returns the billing status of the user's account, e.g. are they still in-trial, actively paying us, or was there a problem with payment.
// - TRIAL_ACTIVE - The trial is still active
// - TRIAL_EXPIRED - The trial has expired
// - SUBSCRIPTION_INACTIVE - The account has a billing account but they are not actively paying us
// - PAYMENT_ERROR - There is a problem with payment
// - ACTIVE - The user is paying us for service and everything is okay
func GetBillingStatus(ctx context.Context, trialInfo trial.Trial, acct *zuora.Account) (grpc.BillingStatus, string, string) {
	// Having days left on the trial means we don't have to care about Zuora.
	if trialInfo.Remaining > 0 {
		return grpc.TRIAL_ACTIVE, "", ""
	}
	// We only create an account for a user after they have added a payment method,
	// so acct == nil is equivalent to "no account on Zuora", which is equivalent to,
	// "they haven't submitted a payment method", which means their trial has expired.
	if acct == nil {
		return grpc.TRIAL_EXPIRED, "", ""
	}
	// Even if the user has an account on Zuora, we can suspend or cancel
	// their account.
	if acct.SubscriptionStatus != zuora.SubscriptionActive {
		return grpc.SUBSCRIPTION_INACTIVE, "", ""
	}
	// At this point, we know the account is active.
	if acct.PaymentStatus.Status != zuora.PaymentOK {
		user.LogWith(ctx, logging.Global()).
			WithField("payment_status", acct.PaymentStatus).
			WithField("zuora_id", acct.ZuoraID).
			WithField("zuora_number", acct.Number).
			Debugf("treating non-active payment status as error")
		return grpc.PAYMENT_ERROR, acct.PaymentStatus.Description, acct.PaymentStatus.Action
	}
	// TODO Future - work out when to use PAYMENT_DUE
	return grpc.ACTIVE, "", ""
}
