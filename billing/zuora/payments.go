package zuora

import "context"

const (
	paymentTokenPath  = "rsa-signatures"
	updatePaymentPath = "payment-methods/credit-cards/%s"
	getPaymentPath    = "payment-methods/credit-cards/accounts/%s"
)

// PaymentStatus is the status of a payment.
type PaymentStatus string

const (
	// PaymentOK means the user's payment details are OK, and they don't need
	// to change them. Also can indicate we haven't tried to use their payment
	// details, and are thus assuming they are OK.
	PaymentOK PaymentStatus = "active"
	// PaymentError means the user needs to sort out their payment details. We
	// return this when there is at least one payment with the `Error` status
	// in the returned Zuora summary.
	PaymentError PaymentStatus = "inactive"
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
func (z *Zuora) GetAuthenticationTokens(ctx context.Context, weaveUserID string) (*AuthenticationTokens, error) {
	resp := &AuthenticationTokens{}
	if weaveUserID != "" {
		resp.AccountNumber = ToZuoraAccountNumber(weaveUserID)
	}

	r, err := z.postJSON(
		ctx,
		paymentTokenPath,
		z.URL(paymentTokenPath), &authenticationRequest{
			URI:    z.cfg.HostedPaymentPageURI,
			Method: "POST",
			PageID: z.cfg.HostedPaymentPageID,
		})
	if err != nil {
		return nil, err
	}
	if err := z.parseJSON(r, resp); err != nil {
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
	r, err := z.putJSON(
		ctx,
		updatePaymentPath,
		z.URL(updatePaymentPath, paymentMethodID),
		&paymentUpdateMethod{DefaultPaymentMethod: true},
	)
	if err != nil {
		return err
	}
	resp := &genericZuoraResponse{}
	if err := z.parseJSON(r, resp); err != nil {
		return err
	}
	if !resp.Success {
		return resp
	}
	return nil
}

// GetPaymentMethod gets the payment method from Zuora.
func (z *Zuora) GetPaymentMethod(ctx context.Context, weaveUserID string) (*CreditCard, error) {
	resp := &paymentMethods{}
	err := z.getJSON(ctx, getPaymentPath, z.URL(getPaymentPath, ToZuoraAccountNumber(weaveUserID)), resp)
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
