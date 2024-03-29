package zuora

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	"github.com/weaveworks/common/logging"
	"github.com/weaveworks/common/user"
	"github.com/weaveworks/service/common/constants/billing"
)

const (
	accountsPath            string = "accounts"
	accountPath             string = accountsPath + "/%s"
	summaryPath             string = accountPath + "/summary"
	subscriptionPath        string = "subscriptions/%s"
	accountSubscriptionPath string = "subscriptions/accounts/%s"
	suspendPath             string = subscriptionPath + "/suspend"
	resumePath              string = subscriptionPath + "/resume"
	deletePath              string = "object/account/%s"

	deleteURL string = "https://rest.apisandbox.zuora.com/v1/" // hard coded (sandbox) because it should not be used in prod

	// BillCycleDay is the day of the month that starts everybody's bills.
	// Current policy is to bill everyone on the 1st of the month
	// We might want to revisit this later, once things are automated
	BillCycleDay int = 1

	dateFormat string = "2006-01-02"
)

// SubscriptionStatus shows the status of a subscription.
type SubscriptionStatus string

const (
	// SubscriptionActive means the subscription is active.
	SubscriptionActive SubscriptionStatus = "active"
	// SubscriptionInactive means the subscription is inactive.
	SubscriptionInactive SubscriptionStatus = "inactive"
)

// Account on Zuora.
type Account struct {
	ZuoraID            string               `json:"id"`
	Number             string               `json:"number"`
	PaymentProviderID  string               `json:"paymentProviderId"`
	SubscriptionStatus SubscriptionStatus   `json:"subscriptionStatus"`
	PaymentStatus      PaymentStatus        `json:"paymentStatus"`
	Subscription       *AccountSubscription `json:"subscription"`
	BillToContact      Contact              `json:"billToContact"`
	BillCycleDay       int                  `json:"billCycleDay"`
}

// AccountSubscription is an account subscription on Zuora.
type AccountSubscription struct {
	SubscriptionNumber    string                 `json:"subscriptionNumber"`
	Currency              string                 `json:"currency"`
	ChargeID              string                 `json:"id"`
	ChargeNumber          string                 `json:"chargeNumber"`
	PricingSummary        string                 `json:"pricingSummary"`
	Price                 float64                `json:"price"`
	SubscriptionStartDate string                 `json:"subscriptionStartDate"`
	RatePlans             []SubscriptionRatePlan `json:"ratePlans"`
}

// ToZuoraAccountNumber converts a weave organization ID to a Zuora Account Number.
//
// It takes the sha256, prefixes with `W`, and truncates it to 32 characters.
// Zuora's requirement is max 50 chars and this number cannot share the same
// prefix as auto-generated numbers (which can be configured and is set to `A`).
// Note that this number differs from the Zuora Account ID but it can be used
// interchangeably in most requests.
func ToZuoraAccountNumber(weaveID string) string {
	h := sha256.New()
	h.Write([]byte(weaveID))
	return fmt.Sprintf("W%x", h.Sum(nil))[:32]
}

// GetAccount gets an account on Zuora.
func (z *Zuora) GetAccount(ctx context.Context, zuoraAccountNumber string) (*Account, error) {
	logger := user.LogWith(ctx, logging.Global())
	if zuoraAccountNumber == "" {
		return nil, ErrInvalidAccountNumber
	}
	zuoraResponse, err := z.getAccountSummary(ctx, zuoraAccountNumber)
	if err != nil {
		return nil, err
	}
	if !zuoraResponse.Success {
		return nil, ErrNotFound
	}
	subscriptionStatus := SubscriptionInactive
	if len(zuoraResponse.Subscriptions) > 0 {
		status := zuoraResponse.Subscriptions[0].Status
		var ok bool
		subscriptionStatus, ok = subscriptionStatusMap[status]
		if !ok {
			logger.Errorf("Unrecognized subscription status: %v", status)
			subscriptionStatus = SubscriptionInactive
		}
	}
	paymentStatus := z.GetPaymentStatus(ctx, zuoraResponse.Payments)
	subscription := &AccountSubscription{}
	subscriptionResponse, err := z.getAccountSubscription(ctx, zuoraAccountNumber)

	if err == nil && zuoraResponse.Success {
		subscription, err = extractNodeSecondsSubscription(ctx, subscriptionResponse.Subscriptions)
		if err != nil {
			logger.Errorf("Failed to find subscription: %v", err)
			return nil, err
		}
	}
	return &Account{
		ZuoraID:            zuoraResponse.BasicInfo.ZuoraID,
		Number:             zuoraAccountNumber,
		PaymentProviderID:  zuoraResponse.BasicInfo.ID,
		SubscriptionStatus: subscriptionStatus,
		PaymentStatus:      paymentStatus,
		Subscription:       subscription,
		BillToContact:      zuoraResponse.BillToContact,
		BillCycleDay:       zuoraResponse.BasicInfo.BillCycleDay,
	}, nil
}

func extractNodeSecondsSubscription(ctx context.Context, subscriptions []subscriptionResponse) (*AccountSubscription, error) {
	var subscription *AccountSubscription
	for _, zuoraSubscription := range subscriptions {
		for _, zuoraPlan := range zuoraSubscription.RatePlans {
			for _, zuoraPlanCharge := range zuoraPlan.RatePlanCharges {
				if strings.HasSuffix(zuoraPlanCharge.Uom, billing.UsageNodeSeconds) {
					if subscription == nil {
						subStartDate, err := time.Parse(dateFormat, zuoraSubscription.SubscriptionStartDate)
						if err != nil {
							return nil, ErrNoSubscriptions
						}
						subscription = &AccountSubscription{
							Currency:              zuoraPlanCharge.Currency,
							ChargeID:              zuoraPlanCharge.ID,
							ChargeNumber:          zuoraPlanCharge.ChargeNumber,
							Price:                 zuoraPlanCharge.Price,
							PricingSummary:        zuoraPlanCharge.PricingSummary,
							SubscriptionNumber:    zuoraSubscription.SubscriptionNumber,
							SubscriptionStartDate: subStartDate.Format(time.RFC3339),
							RatePlans:             zuoraSubscription.RatePlans,
						}
						// I could return, but I am going to check for duplicate UOMs instead
					} else {
						// more than one plan with 'node-secods': fail
						return nil, ErrDuplicateSubscriptions
					}
				}
			}
		}
	}
	return subscription, nil
}

// GetPaymentStatus gets the overall payment status based on the history of Zuora payments.
//
// If the last payment has an error, we treat the overall status as PaymentError.
// If there have been no payments, we assume everything is fine,
// because PaymentError is meant to indicate that the user needs to take
// action, and the user doesn't need to do things just because we haven't got
// around to charging them.
// Since zuora returns dates and not datetimes, if there are multiple latest payments (same date)
// and one of them has an error, treat the overall status as PaymentError.
func (z Zuora) GetPaymentStatus(ctx context.Context, payments []Payment) PaymentStatus {
	logger := user.LogWith(ctx, logging.Global())
	var latestID string
	latestStatus := PaymentOK
	var latestDate time.Time
	for _, payment := range payments {
		effectiveDate, err := time.Parse(dateFormat, payment.EffectiveDate)
		if err != nil {
			logger.
				WithField("date", payment.EffectiveDate).WithField("err", err).
				Errorf("failed to parse payment status effective date")
			return PaymentStatus{Status: PaymentError}
		}

		status, ok := paymentStatusMap[payment.Status]
		if !ok {
			// Zuora returned an unexpected payment status. Assume it's bad.
			logger.WithField("status", payment.Status).Errorf("unrecognized payment status")
			return PaymentStatus{Status: PaymentError}
		}

		if latestDate.IsZero() || latestDate.Before(effectiveDate) {
			logger.
				WithField("old_status", latestStatus).WithField("old_date", latestDate).
				WithField("new_status", status).WithField("new_date", effectiveDate).
				WithField("underlying_zuora_payment_status", payment.Status).Debugf("updating payment status")
			latestID = payment.ID
			latestDate = effectiveDate
			latestStatus = status
		} else if latestDate.Equal(effectiveDate) && status == PaymentError {
			latestStatus = PaymentError
		}
	}
	if latestStatus == PaymentError {
		stripeError := z.getUnderlyingStripePaymentError(ctx, latestID)
		return PaymentStatus{
			Status:      latestStatus,
			Description: stripeError.Description,
			Action:      stripeError.Action,
		}
	}
	return PaymentStatus{Status: latestStatus}
}

func (z Zuora) getUnderlyingStripePaymentError(ctx context.Context, latestID string) StripeError {
	transactionLog, err := z.GetPaymentTransactionLog(ctx, latestID)
	if err != nil {
		user.LogWith(ctx, logging.Global()).WithField("err", err).WithField("id", latestID).Errorf("failed to get transaction log, returning generic error instead")
		return GetStripeError(GenericDecline)
	}
	return GetStripeError(StripeDeclineCode(transactionLog.GatewayReasonCode))
}

var subscriptionStatusMap = map[string]SubscriptionStatus{
	"Draft":             SubscriptionActive,
	"PendingActivation": SubscriptionActive,
	"PendingAcceptance": SubscriptionActive,
	"Active":            SubscriptionActive,
	"Suspended":         SubscriptionInactive,
	"Cancelled":         SubscriptionInactive,
	"Expired":           SubscriptionInactive,
}

var paymentStatusMap = map[string]string{
	"Draft":      PaymentOK,
	"Processing": PaymentOK,
	"Processed":  PaymentOK,
	"Error":      PaymentError,
	"Voided":     PaymentOK, // User doesn't have to do anything about voided payments
	"Canceled":   PaymentOK, // User doesn't have to do anything about canceled payments
	"Posted":     PaymentOK,
}

// Payment represents part of a payment returned from Zuora's getAccountSummary request
type Payment struct {
	ID            string `json:"id"`
	Status        string `json:"status"`
	EffectiveDate string `json:"effectiveDate"`
}

type invoice struct {
	Status string `json:"status"`
}

type zuoraSummaryResponse struct {
	genericZuoraResponse
	BasicInfo struct {
		ZuoraID      string `json:"id"`
		ID           string `json:"accountNumber"`
		Currency     string `json:"currency"`
		BillCycleDay int    `json:"billCycleDay"`
	} `json:"basicInfo"`
	BillToContact Contact `json:"billToContact"`
	Subscriptions []struct {
		ID        string                 `json:"id"`
		Status    string                 `json:"status"`
		RatePlans []SubscriptionRatePlan `json:"ratePlans"`
	} `json:"subscriptions"`
	Payments []Payment `json:"payments"`
	Invoices []invoice `json:"invoices"`
}

// Contact groups contact details in Zuora (typically called "billToContact" in their API).
type Contact struct {
	ID        string `json:"id,omitempty"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Address1  string `json:"address1"`
	Address2  string `json:"address2"`
	City      string `json:"city"`
	ZipCode   string `json:"zipCode"`
	State     string `json:"state"`
	Country   string `json:"country"`
	WorkEmail string `json:"workEmail"`
	WorkPhone string `json:"workPhone"`
}

func (z *Zuora) getAccountSummary(ctx context.Context, zuoraID string) (*zuoraSummaryResponse, error) {
	resp := &zuoraSummaryResponse{}
	err := z.Get(ctx, summaryPath, z.URL(summaryPath, zuoraID), resp)
	return resp, err
}

type zuoraSubscriptionResponse struct {
	genericZuoraResponse
	Subscriptions []subscriptionResponse `json:"subscriptions"`
}

type subscriptionResponse struct {
	SubscriptionNumber    string                 `json:"subscriptionNumber"`
	SubscriptionStartDate string                 `json:"subscriptionStartDate"`
	RatePlans             []SubscriptionRatePlan `json:"ratePlans"`
}

// SubscriptionRatePlan describes the pricing of a subscription
type SubscriptionRatePlan struct {
	ProductID         string `json:"productId"`
	ProductRatePlanID string `json:"productRatePlanId"`
	RatePlanCharges   []struct {
		ID             string  `json:"id"`
		ChargeNumber   string  `json:"number"`
		PricingSummary string  `json:"pricingSummary"`
		Currency       string  `json:"currency"`
		Price          float64 `json:"price"`
		Uom            string  `json:"uom"`
	} `json:"ratePlanCharges"`
}

func (z *Zuora) getAccountSubscription(ctx context.Context, accountNumber string) (*zuoraSubscriptionResponse, error) {
	resp := &zuoraSubscriptionResponse{}
	err := z.Get(ctx, accountSubscriptionPath, z.URL(accountSubscriptionPath, accountNumber), resp)
	return resp, err
}

type updateAccountRequest struct {
	BillToContact Contact `json:"billToContact"`
}

type updateAccountResponse struct {
	genericZuoraResponse
}

// UpdateAccount changes the details on an existing account, and returns the newly updated account.
func (z *Zuora) UpdateAccount(ctx context.Context, zuoraAccountNumber string, userDetails *Account) (*Account, error) {
	if zuoraAccountNumber == "" {
		return nil, ErrInvalidAccountNumber
	}
	currentAccount, err := z.getAccountSummary(ctx, zuoraAccountNumber)
	if err != nil {
		return nil, err
	}
	if len(currentAccount.Subscriptions) == 0 {
		return nil, ErrNoSubscriptions
	}

	resp := &updateAccountResponse{}
	err = z.Put(ctx, accountPath, z.URL(accountPath, zuoraAccountNumber), &updateAccountRequest{
		BillToContact: userDetails.BillToContact,
	}, resp)
	if err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, resp
	}

	// Account suspension happens separately from updating the actual account data.
	// z.resume/suspend update the 'Subscriptions' model, not the 'Account' model.
	if userDetails.SubscriptionStatus != "" {
		switch userDetails.SubscriptionStatus {
		case SubscriptionActive:
			// resume
			if err := z.resumeAccount(ctx, currentAccount.Subscriptions[0].ID); err != nil {
				return nil, err
			}
			break
		case SubscriptionInactive:
			// suspend
			if err := z.suspendAccount(ctx, currentAccount.Subscriptions[0].ID); err != nil {
				return nil, err
			}
			break
		default:
			return nil, ErrInvalidSubscriptionStatus
		}

	}

	return z.GetAccount(ctx, zuoraAccountNumber)
}

type resumeAccountRequest struct {
	// REQUIRED: Must be one of {Today, EndOfLastInvoicePeriod, SpecificDate, FixedPeriodsFromToday}
	ResumePolicy string `json:"resumePolicy"`
}

type resumeAccountResponse struct {
	genericZuoraResponse
	SubscriptionID string  `json:"subscriptionId"`
	ResumeDate     string  `json:"resumeDate"`
	TermEndDate    string  `json:"termEndDate"`
	TotalDeltaTcv  float64 `json:"totalDeltaTcv"`
}

func (z *Zuora) resumeAccount(ctx context.Context, subscriptionID string) error {
	resp := &resumeAccountResponse{}
	err := z.Put(
		ctx,
		resumePath,
		z.URL(resumePath, subscriptionID),
		&resumeAccountRequest{ResumePolicy: z.cfg.ResumePolicy},
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

type suspendAccountRequest struct {
	// REQUIRED: Must be one of {Today, EndOfLastInvoicePeriod, SpecificDate, FixedPeriodsFromToday}
	SuspendPolicy string `json:"suspendPolicy"`
}

type suspendAccountResponse struct {
	resumeAccountResponse
	SuspendDate string `json:"suspendDate"`
	InvoiceID   string `json:"invoiceId"`
}

func (z *Zuora) suspendAccount(ctx context.Context, subscriptionID string) error {
	resp := &suspendAccountResponse{}
	err := z.Put(
		ctx,
		suspendPath,
		z.URL(suspendPath, subscriptionID),
		&suspendAccountRequest{SuspendPolicy: z.cfg.SuspendPolicy},
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

type createAccountRequest struct {
	AccountNumber                string             `json:"accountNumber"` // Max 50 chars. See `ToZuoraAccountNumber()`
	Name                         string             `json:"name"`          // Doesn't matter, but required.
	Currency                     string             `json:"currency"`      // Must be in product currency list. E.g. USD, GBP.
	BillToContact                Contact            `json:"billToContact"`
	HpmCreditCardPaymentMethodID string             `json:"hpmCreditCardPaymentMethodId,omitempty"` // refId from callback from hosted payment method iframe.
	CreditCard                   *accountCreditCard `json:"creditCard,omitempty"`
	Subscription                 *subscription      `json:"subscription"`
	BillCycleDay                 int                `json:"billCycleDay"` // Specify any day of the month (1-31, where 31 = end-of-month), or 0 for auto-set.

	// This field is version controlled in Zuora.
	// We explicitly use major version 1 (which we specify in the url),
	// but not do explicitly use minor versions.
	// Zuora by default uses the lowest minor version available.
	// See https://knowledgecenter.zuora.com/DC_Developers/C_REST_API/A_REST_basics#Zuora_REST_API_Versions
	// and https://knowledgecenter.zuora.com/DC_Developers/C_REST_API/A_REST_basics/Zuora_REST_API_Minor_Version_History
	InvoiceCollect bool `json:"invoiceCollect"` // Specify whether to create an initial invoice when creating the account.
}

// subscription is a type of Zuora product. It says when this subscription started and how long it lasts.
type subscription struct {
	TermType               string      `json:"termType"`              // One of "TERMED" or "EVERGREEN"
	ContractEffectiveDate  string      `json:"contractEffectiveDate"` // In the format yyyy-mm-dd
	SubscribeToRatePlans   []*ratePlan `json:"subscribeToRatePlans"`
	ServiceActivationDate  string      `json:"serviceActivationDate,omitempty"` // In the format yyyy-mm-dd
	CustomerAcceptanceDate string      `json:"customerAcceptanceDate,omitempty"`
}

// ratePlan holds information about how much each unit of measure costs.
type ratePlan struct {
	ProductRatePlanID string `json:"productRatePlanId"` // Id from product list
}

type accountCreditCard struct {
	CardType        string `json:"cardType"`
	CardNumber      string `json:"cardNumber"`
	ExpirationMonth int    `json:"expirationMonth"`
	ExpirationYear  int    `json:"expirationYear"`
}

type createAccountResponse struct {
	genericZuoraResponse
	AccountNumber string `json:"accountNumber"`
	AccountID     string `json:"accountId"`
}

// CreateAccount creates an account.
func (z *Zuora) CreateAccount(ctx context.Context, orgID string, contact Contact, currency, paymentMethodID string, billCycleDay int, serviceActivationTime time.Time) (*Account, error) {
	serviceActivationDate, acceptanceDate := computeActivationAndAcceptanceDate(serviceActivationTime)
	zuoraAccountNumber := ToZuoraAccountNumber(orgID)
	return z.createAccount(ctx, zuoraAccountNumber, &createAccountRequest{
		AccountNumber:                zuoraAccountNumber,
		Name:                         orgID,
		Currency:                     currency,
		BillToContact:                contact,
		HpmCreditCardPaymentMethodID: paymentMethodID,
		Subscription: &subscription{
			TermType:               z.cfg.SubscriptionTermType,
			ContractEffectiveDate:  serviceActivationDate.Format(dateFormat),
			CustomerAcceptanceDate: acceptanceDate.Format(dateFormat),
			SubscribeToRatePlans:   []*ratePlan{{ProductRatePlanID: z.cfg.ProductRatePlanID}},
		},
		BillCycleDay:   billCycleDay,
		InvoiceCollect: false,
	})
}

// CreateAccountWithCC creates an account with credit card details. It is, for now, only used in tests.
func (z *Zuora) CreateAccountWithCC(ctx context.Context, orgID string, contact Contact, currency string, billCycleDay int, cardType, cardNumber string, expirationMonth, expirationYear int, serviceActivationTime time.Time) (*Account, error) {
	serviceActivationDate, acceptanceDate := computeActivationAndAcceptanceDate(serviceActivationTime)
	zuoraAccountNumber := ToZuoraAccountNumber(orgID)
	return z.createAccount(ctx, zuoraAccountNumber, &createAccountRequest{
		AccountNumber: zuoraAccountNumber,
		Name:          orgID,
		Currency:      currency,
		BillToContact: contact,
		CreditCard: &accountCreditCard{
			CardType:        cardType,
			CardNumber:      cardNumber,
			ExpirationMonth: expirationMonth,
			ExpirationYear:  expirationYear,
		},
		Subscription: &subscription{
			TermType:               z.cfg.SubscriptionTermType,
			ContractEffectiveDate:  serviceActivationDate.Format(dateFormat),
			CustomerAcceptanceDate: acceptanceDate.Format(dateFormat),
			SubscribeToRatePlans:   []*ratePlan{{ProductRatePlanID: z.cfg.ProductRatePlanID}},
		},
		BillCycleDay:   billCycleDay,
		InvoiceCollect: false,
	})
}

func computeActivationAndAcceptanceDate(serviceActivationTime time.Time) (time.Time, time.Time) {
	today := time.Now().UTC().Truncate(24 * time.Hour)
	serviceActivationDate := serviceActivationTime.UTC().Truncate(24 * time.Hour)
	acceptanceDate := today
	// If the user is configuring billing before the end of the trial
	if acceptanceDate.Before(serviceActivationDate) {
		// Zuora complains if it's before serviceActivationDate or contractEffectiveDate
		// our policies don't require it, but it's nice to be able to tell the
		// difference between trial ending, and billing configured
		acceptanceDate = serviceActivationDate
	}
	return serviceActivationDate, acceptanceDate
}

// CreateAccount creates a Zuora account.
func (z *Zuora) createAccount(ctx context.Context, zuoraAccountNumber string, request *createAccountRequest) (*Account, error) {
	resp := &createAccountResponse{}
	err := z.Post(
		ctx,
		accountsPath,
		z.URL(accountsPath),
		request,
		resp,
	)
	if err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, resp
	}
	return z.GetAccount(ctx, zuoraAccountNumber)
}

// DeleteAccount deletes a Zuora account.
func (z *Zuora) DeleteAccount(ctx context.Context, zuoraID string) error {
	path := deleteURL + fmt.Sprintf(deletePath, zuoraID)
	user.LogWith(ctx, logging.Global()).Warnf("Deleting account %v", zuoraID)
	return z.Delete(ctx, deletePath, path, nil)
}
