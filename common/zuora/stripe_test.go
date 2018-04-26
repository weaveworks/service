package zuora_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/weaveworks/service/common/zuora"
)

func TestGetStripeErrorForKnownCodeShouldReturnTheCorrespondingError(t *testing.T) {
	assert.Equal(t, zuora.StripeError{
		Code:        zuora.DoNotHonor,
		Description: "The card has been declined for an unknown reason.",
		Action:      "Please contact your card issuer for more information.",
	}, zuora.GetStripeError(zuora.StripeDeclineCode("do_not_honor")))
}

func TestGetStripeErrorForFraudulentPaymentShouldReturnGenericDecline(t *testing.T) {
	genericDecline := zuora.StripeError{
		Code:        zuora.GenericDecline,
		Description: "The card has been declined for an unknown reason.",
		Action:      "Please contact your card issuer for more information.",
	}
	assert.Equal(t, genericDecline, zuora.GetStripeError(zuora.StripeDeclineCode("fraudulent")))
	assert.Equal(t, genericDecline, zuora.GetStripeError(zuora.StripeDeclineCode("lost_card")))
	assert.Equal(t, genericDecline, zuora.GetStripeError(zuora.StripeDeclineCode("stolen_card")))
}

func TestGetStripeErrorForUnknownCodeShouldReturnGenericDecline(t *testing.T) {
	assert.Equal(t, zuora.StripeError{
		Code:        zuora.GenericDecline,
		Description: "The card has been declined for an unknown reason.",
		Action:      "Please contact your card issuer for more information.",
	}, zuora.GetStripeError(zuora.StripeDeclineCode("foo")))
}
