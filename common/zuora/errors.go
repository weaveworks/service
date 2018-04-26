package zuora

import (
	"fmt"
	"strings"
)

var (
	// ErrInvalidSubscriptionStatus means we got a subscription status we don't recognize.
	ErrInvalidSubscriptionStatus = fmt.Errorf(
		"invalid subscription status, should be one of: %v",
		[]SubscriptionStatus{SubscriptionActive, SubscriptionInactive},
	)
	// ErrNoDefaultPaymentMethod means we couldn't find a default payment method.
	ErrNoDefaultPaymentMethod = fmt.Errorf("no default payment method found")
	// ErrorObtainingPaymentMethod means we couldn't look up a payment method.
	ErrorObtainingPaymentMethod = fmt.Errorf("error looking up payment method")
	// ErrNoSubscriptions means we couldn't find subscriptions.
	ErrNoSubscriptions = fmt.Errorf("no subscriptions")
	// ErrNotFound means we couldn't find what we were looking for.
	ErrNotFound = fmt.Errorf("not found")
	// ErrInvalidPaymentID means the zuora payment ID is invalid, most likely blank
	ErrInvalidPaymentID = fmt.Errorf("invalid zuora payment ID")
	// ErrTooManyTransactions means we got more transactions than we expected.
	ErrTooManyTransactions = fmt.Errorf("too many transactions")
	// ErrDuplicateSubscriptions means we found too many rate plans with the same Uom
	ErrDuplicateSubscriptions = fmt.Errorf("duplicate subscriptions")
	// ErrInvalidAccountNumber means the zuora account number is invalid, most likely blank
	ErrInvalidAccountNumber = fmt.Errorf("invalid zuora account number")
)

const (
	// ErrRuleRestrictionInRestService has, in our experience, been returned by Zuora when
	// we try to create an invoice but there is no chargeable usage. However, according to the
	// official documentation (https://knowledgecenter.zuora.com/DC_Developers/C_REST_API/A_REST_basics/3_Responses_and_errors),
	// 5 means "error in the REST service", 30 means "rule restriction" (so presumably some
	// sort of input validation in Zuora's business logic), and 40000 is supposed to refer to
	// one of Zuora's concepts/objects, but as of writing this comment, 40000 is undocumented.
	// More details in billing/issues/223 and billing/issues/226.
	ErrRuleRestrictionInRestService int = 54000030

	// NoInvoiceOnWhichToCollectPayment has, in our experience, been returned by Zuora when
	// we try to create an invoice but there is no chargeable usage.
	// In such cases, Zuora returns a 200/OK, but actually fails with a response containing:
	//     "code" : 54000030,
	//     "message" : "There is no invoice on which to collect payment."
	// More details in billing/issues/223 and billing/issues/226.
	NoInvoiceOnWhichToCollectPayment string = "There is no invoice on which to collect payment"

	// ErrInternalErrorInRestService has, in our experience, been returned by Zuora when
	// we try to create an invoice but the chargeable usage is less than $0.50. However,
	// according to the official documentation (https://knowledgecenter.zuora.com/DC_Developers/C_REST_API/A_REST_basics/3_Responses_and_errors),
	// 5 means "error in the REST service", 60 means "internal error" (so it could pretty much
	// be anything wrong on Zuora's end), and 40000 is supposed to refer to one of Zuora's
	// concepts/objects, but as of writing this comment, 40000 is undocumented.
	// More details in billing/issues/240.
	ErrInternalErrorInRestService int = 54000060

	// AmountMustBeAtLeast50Cents has, in our experience, been returned by Zuora when
	// we try to create an invoice but the chargeable usage is less than $0.50.
	// In such cases, Zuora returns a 200/OK, but actually fails with a response containing:
	//     "code" : 54000060,
	//     "message" : "Error processing transaction.invalid_request_erro - Amount must be at least 50 cents"
	// More details in billing/issues/240.
	AmountMustBeAtLeast50Cents string = "Amount must be at least 50 cents"
)

// NoChargeableUsage returns true if err is due to no chargeable usage.
//
// We check both the error code and the error message, although fragile, because error codes
// are either very generic or not fully documented as of writing this comment. See also:
// - comments on the constants used for these checks, in zuora/errors.go.
// - https://knowledgecenter.zuora.com/DC_Developers/C_REST_API/A_REST_basics/3_Responses_and_errors
func (z *Zuora) NoChargeableUsage(err error) bool {
	return z.ContainsErrorCode(err, ErrRuleRestrictionInRestService) &&
		strings.Contains(err.Error(), NoInvoiceOnWhichToCollectPayment)
}

// ChargeableUsageTooLow returns true if err is due to chargeable usage too low.
//
// We check both the error code and the error message, although fragile, because error codes
// are either very generic or not fully documented as of writing this comment. See also:
// - comments on the constants used for these checks, in zuora/errors.go.
// - https://knowledgecenter.zuora.com/DC_Developers/C_REST_API/A_REST_basics/3_Responses_and_errors
func (z *Zuora) ChargeableUsageTooLow(err error) bool {
	return z.ContainsErrorCode(err, ErrInternalErrorInRestService) &&
		strings.Contains(err.Error(), AmountMustBeAtLeast50Cents)
}
