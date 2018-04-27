package zuora

import (
	"context"
	"fmt"
)

const (
	paymentTokenPath                  = "rsa-signatures"
	updatePaymentPath                 = "payment-methods/credit-cards/%s"
	getPaymentPath                    = "payment-methods/credit-cards/accounts/%s"
	getPaymentsPath                   = "transactions/payments/accounts/%s"
	actionQueryPath                   = "action/query"
	paymentTransactionLogZOQLTemplate = "SELECT Id, Gateway, GatewayState, GatewayReasonCode, GatewayReasonCodeDescription, GatewayTransactionType, PaymentId, TransactionId, TransactionDate, RequestString, ResponseString FROM PaymentTransactionLog WHERE PaymentId='%v'"
)

// PaymentStatus is the status of a payment.
type PaymentStatus struct {
	Status      string `json:"status"`
	Description string `json:"description"`
	Action      string `json:"action"`
}

const (
	// PaymentOK means the user's payment details are OK, and they don't need
	// to change them. Also can indicate we haven't tried to use their payment
	// details, and are thus assuming they are OK.
	PaymentOK = "active"
	// PaymentError means the user needs to sort out their payment details. We
	// return this when there is at least one payment with the `Error` status
	// in the returned Zuora summary.
	PaymentError = "inactive"
)

// AuthenticationTokens are authentication tokens from Zuora.
type AuthenticationTokens struct {
	// Response fields from the token call.
	genericZuoraResponse
	Signature string `json:"signature"`
	Token     string `json:"token"`
	TenantID  string `json:"tenantId"`
	Key       string `json:"key"`

	// Additional fields not being part of the token response but we pass
	// this on to the requester to be used for Zuora's Hosted Payment Pages.
	// See https://knowledgecenter.zuora.com/CA_Commerce/T_Hosted_Commerce_Pages/B_Payment_Pages_2.0/J_Client_Parameters_for_Payment_Pages_2.0
	// for an explanation on parameters.
	AccountNumber        string `json:"field_accountId,omitempty"` // Note this is the Account Number
	HostedPaymentPageID  string `json:"id"`
	HostedPaymentPageURL string `json:"url"`
	PaymentGateway       string `json:"paymentGateway"`       // Name of the payment gateway
	SupportedCards       string `json:"param_supportedTypes"` // Comma-separated list
}

type authenticationRequest struct {
	URI    string `json:"uri"`
	Method string `json:"method"`
	PageID string `json:"pageId"`
}

// GetAuthenticationTokens gets authentication tokens from Zuora.
func (z *Zuora) GetAuthenticationTokens(ctx context.Context, zuoraAccountNumber string) (*AuthenticationTokens, error) {
	resp := &AuthenticationTokens{}
	if zuoraAccountNumber != "" {
		resp.AccountNumber = zuoraAccountNumber
	}

	err := z.Post(
		ctx,
		paymentTokenPath,
		z.URL(paymentTokenPath), &authenticationRequest{
			URI:    z.cfg.HostedPaymentPageURI,
			Method: "POST",
			PageID: z.cfg.HostedPaymentPageID,
		},
		resp,
	)
	if err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, resp
	}

	resp.HostedPaymentPageID = z.cfg.HostedPaymentPageID
	resp.HostedPaymentPageURL = z.cfg.HostedPaymentPageURI
	resp.PaymentGateway = z.cfg.PaymentGateway
	resp.SupportedCards = z.cfg.SupportedCards
	return resp, nil
}

type paymentUpdateMethod struct {
	DefaultPaymentMethod bool `json:"defaultPaymentMethod"`
}

// UpdatePaymentMethod updates the payment method.
func (z *Zuora) UpdatePaymentMethod(ctx context.Context, paymentMethodID string) error {
	resp := &genericZuoraResponse{}
	err := z.Put(
		ctx,
		updatePaymentPath,
		z.URL(updatePaymentPath, paymentMethodID),
		&paymentUpdateMethod{DefaultPaymentMethod: true},
		resp,
	)
	if err != nil {
		return err
	}
	if !resp.Success {
		return resp
	}
	return nil
}

// GetPaymentMethod gets the payment method from Zuora.
func (z *Zuora) GetPaymentMethod(ctx context.Context, zuoraAccountNumber string) (*CreditCard, error) {
	if zuoraAccountNumber == "" {
		return nil, ErrInvalidAccountNumber
	}
	resp := &paymentMethods{}
	err := z.Get(ctx, getPaymentPath, z.URL(getPaymentPath, zuoraAccountNumber), resp)
	if err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, resp
	}

	// Find default payment method and return that
	for _, card := range resp.CreditCards {
		if card.DefaultPaymentMethod {
			return &card, nil
		}
	}
	return nil, ErrNoDefaultPaymentMethod
}

type paymentMethods struct {
	genericZuoraResponse
	CreditCards []CreditCard `json:"creditCards"`
}

// CreditCard information.
type CreditCard struct {
	ID                   string `json:"id"`
	DefaultPaymentMethod bool   `json:"defaultPaymentMethod"`
	CardType             string `json:"cardType"`
	CardNumber           string `json:"cardNumber"`
	ExpirationMonth      int    `json:"expirationMonth"`
	ExpirationYear       int    `json:"expirationYear"`
	CardHolderInfo       struct {
		CardHolderName string `json:"cardHolderName"`
		AddressLine1   string `json:"addressLine1"`
		AddressLine2   string `json:"addressLine2"`
		City           string `json:"city"`
		State          string `json:"state"`
		Country        string `json:"country"`
		Phone          string `json:"phone"`
		Email          string `json:"email"`
		ZipCode        string `json:"zipCode"`
	} `json:"cardHolderInfo"`
}

// GetPayments returns payments.
func (z *Zuora) GetPayments(ctx context.Context, zuoraAccountNumber string) ([]*PaymentDetails, error) {
	if zuoraAccountNumber == "" {
		return nil, ErrInvalidAccountNumber
	}
	resp := &paymentsResponse{}
	err := z.Get(ctx, getPaymentsPath, z.URL(getPaymentsPath, zuoraAccountNumber), resp)
	if err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, resp
	}
	return resp.PaymentDetails, nil
}

type paymentsResponse struct {
	genericZuoraResponse
	PaymentDetails []*PaymentDetails `json:"payments"`
}

// PaymentDetails represents... details on a payment!
type PaymentDetails struct {
	ID                       string        `json:"id"`
	AccountID                string        `json:"accountID"`
	AccountNumber            string        `json:"accountNumber"`
	Type                     string        `json:"type"`
	EffectiveDate            string        `json:"effectiveDate"`
	PaymentNumber            string        `json:"paymentNumber"`
	PaymentMethodID          string        `json:"paymentMethodID"`
	Amount                   float64       `json:"amount"`
	PaidInvoices             []PaidInvoice `json:"paidInvoices"`
	GatewayTransactionNumber string        `json:"gatewayTransactionNumber"`
	Status                   string        `json:"status"`
}

// PaidInvoice represents... a paid invoice!
type PaidInvoice struct {
	InvoiceID            string  `json:"invoiceID"`
	InvoiceNumber        string  `json:"invoiceNumber"`
	AppliedPaymentAmount float64 `json:"appliedPaymentAmount"`
}

// GetPaymentTransactionLog retrieves details on a payment transaction. Among other things, it allows us to get details on payment errors from the underlying payment gateway.
func (z *Zuora) GetPaymentTransactionLog(ctx context.Context, paymentID string) (*PaymentTransaction, error) {
	if paymentID == "" {
		return nil, ErrInvalidPaymentID
	}
	req := &actionQueryRequest{QueryString: fmt.Sprintf(paymentTransactionLogZOQLTemplate, paymentID)}
	resp := &actionQueryResponse{}
	err := z.Post(ctx, actionQueryPath, z.RestURL(actionQueryPath), req, resp)
	if err != nil {
		return nil, err
	}
	if len(resp.Records) == 0 {
		return nil, ErrNotFound
	}
	if len(resp.Records) > 1 {
		return nil, ErrTooManyTransactions
	}
	return resp.Records[0], nil
}

type actionQueryRequest struct {
	QueryString string `json:"queryString"`
}

type actionQueryResponse struct {
	Done    bool                  `json:"done"`
	Size    int                   `json:"size"`
	Records []*PaymentTransaction `json:"records"`
}

// PaymentTransaction gathers details on a transaction done via a specific payment method.
type PaymentTransaction struct {
	ID                           string `json:"Id"`
	Gateway                      string `json:"Gateway"`
	GatewayState                 string `json:"GatewayState"`
	GatewayReasonCode            string `json:"GatewayReasonCode"`
	GatewayReasonCodeDescription string `json:"GatewayReasonCodeDescription"`
	GatewayTransactionType       string `json:"GatewayTransactionType"`
	PaymentID                    string `json:"PaymentId"`
	TransactionDate              string `json:"TransactionDate"`
	TransactionID                string `json:"TransactionId"`
	RequestString                string `json:"RequestString"`
	ResponseString               string `json:"ResponseString"`
}
