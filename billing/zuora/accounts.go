package zuora

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	"github.com/weaveworks/common/logging"
	"github.com/weaveworks/service/billing/util"
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
	BillToContact      *contact             `json:"billToContact"`
	BillCycleDay       int                  `json:"billCycleDay"`
}

// AccountSubscription is an account subscription on Zuora.
type AccountSubscription struct {
	SubscriptionNumber    string  `json:"subscriptionNumber"`
	Currency              string  `json:"currency"`
	ChargeID              string  `json:"id"`
	ChargeNumber          string  `json:"chargeNumber"`
	PricingSummary        string  `json:"pricingSummary"`
	Price                 float64 `json:"price"`
	SubscriptionStartDate string  `json:"subscriptionStartDate"`
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
func (z *Zuora) GetAccount(ctx context.Context, weaveUserID string) (*Account, error) {
	logger := logging.With(ctx)
	accountNumber := ToZuoraAccountNumber(weaveUserID)
	zuoraResponse, err := z.getAccountSummary(ctx, accountNumber)
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
			logging.With(ctx).Errorf("Unrecognized subscription status: %v", status)
			subscriptionStatus = SubscriptionInactive
		}
	}
	paymentStatus := getPaymentStatus(ctx, zuoraResponse.Payments)
	subscription := &AccountSubscription{}
	subscriptionResponse, err := z.getAccountSubscription(ctx, accountNumber)

	if err == nil && zuoraResponse.Success {
		subscription, err = extractNodeSecondsSubscription(ctx, subscriptionResponse.Subscriptions)
		if err != nil {
			logger.Errorf("Failed to find subscription: %v", err)
			return nil, err
		}
	}
	return &Account{
		ZuoraID:            zuoraResponse.BasicInfo.ZuoraID,
		Number:             accountNumber,
		PaymentProviderID:  zuoraResponse.BasicInfo.ID,
		SubscriptionStatus: subscriptionStatus,
		PaymentStatus:      paymentStatus,
		Subscription:       subscription,
		BillToContact: &contact{
			FirstName: zuoraResponse.BillToContact.FirstName,
			LastName:  zuoraResponse.BillToContact.LastName,
			Country:   zuoraResponse.BillToContact.Country,
			State:     zuoraResponse.BillToContact.State,
			WorkEmail: zuoraResponse.BillToContact.WorkEmail,
		},
		BillCycleDay: zuoraResponse.BasicInfo.BillCycleDay,
	}, nil
}

func extractNodeSecondsSubscription(ctx context.Context, subscriptions []subscriptionResponse) (*AccountSubscription, error) {
	var subscription *AccountSubscription
	for _, zuoraSubscription := range subscriptions {
		for _, zuoraPlans := range zuoraSubscription.RatePlans {
			for _, zuoraPlan := range zuoraPlans.RatePlanCharges {
				if strings.HasSuffix(zuoraPlan.Uom, util.UsageNodeSeconds) {
					if subscription == nil {
						subStartDate, err := time.Parse("2006-01-02", zuoraSubscription.SubscriptionStartDate)
						if err != nil {
							return nil, ErrNoSubscriptions
						}
						subscription = &AccountSubscription{
							Currency:              zuoraPlan.Currency,
							ChargeID:              zuoraPlan.ID,
							ChargeNumber:          zuoraPlan.ChargeNumber,
							Price:                 zuoraPlan.Price,
							PricingSummary:        zuoraPlan.PricingSummary,
							SubscriptionNumber:    zuoraSubscription.SubscriptionNumber,
							SubscriptionStartDate: subStartDate.Format(time.RFC3339),
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

// getPaymentStatus gets the overall payment status based on the history of Zuora payments.
//
// If any one payment has an error, we treat the overall status as
// PaymentError. If there have been no payments, we assume everything is fine,
// because PaymentError is meant to indicate that the user needs to take
// action, and the user doesn't need to do things just because we haven't got
// around to charging them.
func getPaymentStatus(ctx context.Context, payments []payment) PaymentStatus {
	for _, payment := range payments {
		status, ok := paymentStatusMap[payment.Status]
		if !ok {
			// Zuora returned an unexpected payment status. Assume it's bad.
			logging.With(ctx).Errorf("Unrecognized payment status: %v", payment.Status)
			return PaymentError
		}
		if status == PaymentError {
			return PaymentError
		}
	}
	return PaymentOK
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

var paymentStatusMap = map[string]PaymentStatus{
	"Draft":      PaymentOK,
	"Processing": PaymentOK,
	"Processed":  PaymentOK,
	"Error":      PaymentError,
	"Voided":     PaymentOK, // User doesn't have to do anything about voided payments
	"Canceled":   PaymentOK, // User doesn't have to do anything about canceled payments
	"Posted":     PaymentOK,
}

type payment struct {
	Status string `json:"status"`
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
	BillToContact struct {
		ID        string `json:"id"`
		FirstName string `json:"firstName"`
		LastName  string `json:"lastName"`
		Country   string `json:"country"`
		State     string `json:"state"`
		WorkEmail string `json:"workEmail"`
	} `json:"billToContact"`
	Subscriptions []struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	} `json:"subscriptions"`
	Payments []payment `json:"payments"`
	Invoices []invoice `json:"invoices"`
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
	SubscriptionNumber    string `json:"subscriptionNumber"`
	SubscriptionStartDate string `json:"subscriptionStartDate"`
	RatePlans             []struct {
		RatePlanCharges []struct {
			ID             string  `json:"id"`
			ChargeNumber   string  `json:"number"`
			PricingSummary string  `json:"pricingSummary"`
			Currency       string  `json:"currency"`
			Price          float64 `json:"price"`
			Uom            string  `json:"uom"`
		} `json:"ratePlanCharges"`
	} `json:"ratePlans"`
}

func (z *Zuora) getAccountSubscription(ctx context.Context, accountNumber string) (*zuoraSubscriptionResponse, error) {
	resp := &zuoraSubscriptionResponse{}
	err := z.Get(ctx, accountSubscriptionPath, z.URL(accountSubscriptionPath, accountNumber), resp)
	return resp, err
}

type updateAccountRequest struct {
	BillToContact *contact `json:"billToContact"`
}

type updateAccountResponse struct {
	genericZuoraResponse
}

// UpdateAccount changes the details on an existing account, and returns the newly updated account.
func (z *Zuora) UpdateAccount(ctx context.Context, id string, userDetails *Account) (*Account, error) {
	currentAccount, err := z.getAccountSummary(ctx, ToZuoraAccountNumber(id))
	if err != nil {
		return nil, err
	}
	if len(currentAccount.Subscriptions) == 0 {
		return nil, ErrNoSubscriptions
	}

	resp := &updateAccountResponse{}
	err = z.Put(ctx, accountPath, z.URL(accountPath, ToZuoraAccountNumber(id)), &updateAccountRequest{
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

	return z.GetAccount(ctx, id)
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
	BillToContact                *contact           `json:"billToContact"`
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

// contact is Zuora's representation of a contact
type contact struct {
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Country   string `json:"country"`
	State     string `json:"state"`
	WorkEmail string `json:"workEmail"` // Email address that account issues are emailed to
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
func (z *Zuora) CreateAccount(ctx context.Context, orgID, currency, firstName, lastName, country, email, state, paymentMethodID string, billCycleDay int, serviceActivationTime time.Time) (*Account, error) {
	serviceActivationDate, acceptanceDate := computeActivationAndAcceptanceDate(serviceActivationTime)
	return z.createAccount(ctx, orgID, &createAccountRequest{
		AccountNumber: ToZuoraAccountNumber(orgID),
		Name:          orgID,
		Currency:      currency,
		BillToContact: &contact{
			FirstName: firstName,
			LastName:  lastName,
			Country:   country,
			State:     state,
			WorkEmail: email,
		},
		HpmCreditCardPaymentMethodID: paymentMethodID,
		Subscription: &subscription{
			TermType:               z.cfg.SubscriptionTermType,
			ContractEffectiveDate:  serviceActivationDate.Format("2006-01-02"),
			CustomerAcceptanceDate: acceptanceDate.Format("2006-01-02"),
			SubscribeToRatePlans:   []*ratePlan{{ProductRatePlanID: z.cfg.SubscriptionPlanID}},
		},
		BillCycleDay:   billCycleDay,
		InvoiceCollect: false,
	})
}

// CreateAccountWithCC creates an account with credit card details. It is, for now, only used in tests.
func (z *Zuora) CreateAccountWithCC(ctx context.Context, orgID, currency, firstName, lastName, country, email, state string, billCycleDay int, cardType, cardNumber string, expirationMonth, expirationYear int, serviceActivationTime time.Time) (*Account, error) {
	serviceActivationDate, acceptanceDate := computeActivationAndAcceptanceDate(serviceActivationTime)
	return z.createAccount(ctx, orgID, &createAccountRequest{
		AccountNumber: ToZuoraAccountNumber(orgID),
		Name:          orgID,
		Currency:      currency,
		BillToContact: &contact{
			FirstName: firstName,
			LastName:  lastName,
			Country:   country,
			State:     state,
			WorkEmail: email,
		},
		CreditCard: &accountCreditCard{
			CardType:        cardType,
			CardNumber:      cardNumber,
			ExpirationMonth: expirationMonth,
			ExpirationYear:  expirationYear,
		},
		Subscription: &subscription{
			TermType:               z.cfg.SubscriptionTermType,
			ContractEffectiveDate:  serviceActivationDate.Format("2006-01-02"),
			CustomerAcceptanceDate: acceptanceDate.Format("2006-01-02"),
			SubscribeToRatePlans:   []*ratePlan{{ProductRatePlanID: z.cfg.SubscriptionPlanID}},
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
func (z *Zuora) createAccount(ctx context.Context, orgID string, request *createAccountRequest) (*Account, error) {
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
	return z.GetAccount(ctx, orgID)
}

// DeleteAccount deletes a Zuora account.
func (z *Zuora) DeleteAccount(ctx context.Context, zuoraID string) error {
	logger := logging.With(ctx)
	path := deleteURL + fmt.Sprintf(deletePath, zuoraID)
	logger.Warningf("Deleting account %v", zuoraID)
	return z.Delete(ctx, deletePath, path, nil)
}
