package zuora_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/weaveworks/service/common/zuora"
)

const mockGetPaymentsResponse = `{
  "payments": [
    {
      "id": "2c92a0b362cd4af40162e06de08d3aed",
      "accountId": "2c92a0fc616ef6210161be3474bd5366",
      "accountNumber": "W7f3da2d2d835a487c12adce80da7de6",
      "accountName": "withered-sound-95",
      "type": "Electronic",
      "effectiveDate": "2018-04-19",
      "paymentNumber": "P-00000018",
      "paymentMethodId": "2c92a0fe61b4023f0161be3472c501f7",
      "amount": 201.030000000,
      "paidInvoices": [
        {
          "invoiceId": "2c92a0aa61c6a44c0161e394498c6cea",
          "invoiceNumber": "INV00000037",
          "appliedPaymentAmount": 201.030000000
        }
      ],
      "gatewayTransactionNumber": null,
      "status": "Error"
    }
  ],
  "success": true
}
`

const mockGetPaymentTransactionLogResponse = `{
  "records": [
    {
      "GatewayReasonCodeDescription": "Your card was declined.",
      "RequestString": "{Request = [amount=20103&currency=USD&metadata[zpayment_number]=P-00000018&card[number]=554001******4013&card[exp_month]=11&card[exp_year]=2020&card[name]=Eulalia Grau&card[address_line1]=Carrer Aragó 182, piso 6&card[address_city]=Barcelona&card[address_state]=Spain&card[address_country]=ESP&card[address_zip]=08011&capture=true], url = [https://api.stripe.com/v1/charges]}",
      "PaymentId": "2c92a0b362cd4af40162e06de08d3aed",
      "GatewayState": "NotSubmitted",
      "Id": "2c92a0b362cd4af40162e06df01f3af1",
      "ResponseString": "[body={\n  \"error\": {\n    \"charge\": \"ch_1CIn0nGugdNUCnXwG1p3D0Ps\",\n    \"code\": \"card_declined\",\n    \"decline_code\": \"do_not_honor\",\n    \"doc_url\": \"https://stripe.com/docs/error-codes/card-declined\",\n    \"message\": \"Your card was declined.\",\n    \"type\": \"card_error\"\n  }\n}\n, charge_status_code=402, ]",
      "TransactionDate": "2018-04-19T17:22:04.000-07:00",
      "Gateway": "stripe",
      "GatewayTransactionType": "Sale",
      "GatewayReasonCode": "do_not_honor"
    }
  ],
  "size": 1,
  "done": true
}`

func TestGetPaymentTransactionLog(t *testing.T) {
	client := zuora.New(conf, &mockClient{
		mockResponses: []mockResponse{
			{body: mockGetPaymentsResponse},
			{body: mockGetPaymentTransactionLogResponse},
		},
	})
	account := "W7f3da2d2d835a487c12adce80da7de6"

	payments, err := client.GetPayments(context.Background(), account)
	assert.Nil(t, err)
	assert.NotNil(t, payments)
	assert.Equal(t, 1, len(payments))
	payment := payments[0]
	assert.Equal(t, &zuora.PaymentDetails{
		ID:              "2c92a0b362cd4af40162e06de08d3aed",
		AccountID:       "2c92a0fc616ef6210161be3474bd5366",
		AccountNumber:   "W7f3da2d2d835a487c12adce80da7de6",
		Type:            "Electronic",
		EffectiveDate:   "2018-04-19",
		PaymentNumber:   "P-00000018",
		PaymentMethodID: "2c92a0fe61b4023f0161be3472c501f7",
		Amount:          201.03,
		PaidInvoices: []zuora.PaidInvoice{
			{
				InvoiceID:            "2c92a0aa61c6a44c0161e394498c6cea",
				InvoiceNumber:        "INV00000037",
				AppliedPaymentAmount: 201.03,
			},
		},
		GatewayTransactionNumber: "",
		Status: "Error",
	}, payment)

	txLog, err := client.GetPaymentTransactionLog(context.Background(), payment.ID)
	assert.Nil(t, err)
	assert.NotNil(t, txLog)
	assert.Equal(t, 1, len(txLog))
	tx := txLog[0]
	assert.Equal(t, &zuora.PaymentTransaction{
		ID:                           "2c92a0b362cd4af40162e06df01f3af1",
		Gateway:                      "stripe",
		GatewayState:                 "NotSubmitted",
		GatewayReasonCode:            "do_not_honor",
		GatewayReasonCodeDescription: "Your card was declined.",
		GatewayTransactionType:       "Sale",
		PaymentID:                    "2c92a0b362cd4af40162e06de08d3aed",
		TransactionDate:              "2018-04-19T17:22:04.000-07:00",
		RequestString:                "{Request = [amount=20103&currency=USD&metadata[zpayment_number]=P-00000018&card[number]=554001******4013&card[exp_month]=11&card[exp_year]=2020&card[name]=Eulalia Grau&card[address_line1]=Carrer Aragó 182, piso 6&card[address_city]=Barcelona&card[address_state]=Spain&card[address_country]=ESP&card[address_zip]=08011&capture=true], url = [https://api.stripe.com/v1/charges]}",
		ResponseString:               "[body={\n  \"error\": {\n    \"charge\": \"ch_1CIn0nGugdNUCnXwG1p3D0Ps\",\n    \"code\": \"card_declined\",\n    \"decline_code\": \"do_not_honor\",\n    \"doc_url\": \"https://stripe.com/docs/error-codes/card-declined\",\n    \"message\": \"Your card was declined.\",\n    \"type\": \"card_error\"\n  }\n}\n, charge_status_code=402, ]",
	}, tx)
}
