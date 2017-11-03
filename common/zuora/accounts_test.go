package zuora_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/weaveworks/service/common/zuora"
)

func TestToZuoraAccountNumber(t *testing.T) {
	assert.Equal(t, "W07a5fd8c403976ced4e81b7da61f31d", zuora.ToZuoraAccountNumber("foo-moo-99"))
}

func TestPaymentStatus(t *testing.T) {
	ctx := context.TODO()

	payments := []zuora.Payment{
		{Status: "Error", EffectiveDate: "2017-11-01"},
		{Status: "Processed", EffectiveDate: "2017-11-02"},
	}
	status := zuora.GetPaymentStatus(ctx, payments)
	assert.Equal(t, status, zuora.PaymentOK)

	payments = []zuora.Payment{
		{Status: "Error", EffectiveDate: "2017-11-02"},
		{Status: "Processed", EffectiveDate: "2017-11-01"},
	}
	status = zuora.GetPaymentStatus(ctx, payments)
	assert.Equal(t, status, zuora.PaymentError)

	payments = []zuora.Payment{
		{Status: "Processed", EffectiveDate: "2017-11-40"}, // failing to parse date on purpose
		{Status: "Error", EffectiveDate: "2017-11-01"},
	}
	status = zuora.GetPaymentStatus(ctx, payments)
	assert.Equal(t, status, zuora.PaymentError)

	payments = []zuora.Payment{
		{Status: "Error", EffectiveDate: "2017-11-01"},
		{Status: "Processed", EffectiveDate: "2017-11-01"},
	}
	status = zuora.GetPaymentStatus(ctx, payments)
	assert.Equal(t, status, zuora.PaymentError)

	payments = []zuora.Payment{
		{Status: "Processed", EffectiveDate: "2017-11-01"},
		{Status: "Error", EffectiveDate: "2017-11-01"},
	}
	status = zuora.GetPaymentStatus(ctx, payments)
	assert.Equal(t, status, zuora.PaymentError)

	payments = []zuora.Payment{
		{Status: "Error", EffectiveDate: "2017-11-01"},
		{Status: "Processed", EffectiveDate: "2017-11-01"},
		{Status: "Processed", EffectiveDate: "2017-10-01"},
	}
	status = zuora.GetPaymentStatus(ctx, payments)
	assert.Equal(t, status, zuora.PaymentError)

	payments = []zuora.Payment{
		{Status: "Error", EffectiveDate: "2017-11-01"},
		{Status: "Processed", EffectiveDate: "2017-10-20"},
		{Status: "Processed", EffectiveDate: "2017-10-01"},
		{Status: "Draft", EffectiveDate: "2017-10-16"},
		{Status: "Voided", EffectiveDate: "2017-10-05"},
	}
	status = zuora.GetPaymentStatus(ctx, payments)
	assert.Equal(t, status, zuora.PaymentError)
}
