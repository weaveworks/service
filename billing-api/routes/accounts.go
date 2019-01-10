package routes

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/weaveworks/service/billing-api/api"

	"github.com/gorilla/mux"

	"github.com/weaveworks/common/logging"
	"github.com/weaveworks/common/user"
	"github.com/weaveworks/service/billing-api/db"
	"github.com/weaveworks/service/billing-api/trial"
	"github.com/weaveworks/service/common/billing/grpc"
	"github.com/weaveworks/service/common/constants/billing"
	"github.com/weaveworks/service/common/orgs"
	"github.com/weaveworks/service/common/render"
	timeutil "github.com/weaveworks/service/common/time"
	"github.com/weaveworks/service/common/zuora"
	"github.com/weaveworks/service/users"
)

const dayTimeLayout = "2006-01-02"

type createAccountRequest struct {
	WeaveID            string        `json:"id"`
	Currency           string        `json:"currency"`
	BillToContact      zuora.Contact `json:"billToContact"`
	PaymentMethodID    string        `json:"paymentMethodId"`
	SubscriptionPlanID string        `json:"subscriptionPlanId"`
}

func (a *API) createAccount(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	logger := user.LogWith(ctx, logging.Global())
	req, err := a.deserializeCreateAccountRequest(logger, r)
	if err != nil {
		return err
	}
	externalID := req.WeaveID
	resp, err := a.getOrganization(ctx, externalID)
	if err != nil {
		return err
	}
	account, err := a.createZuoraAccount(r.Context(), logger, req, resp)
	if err != nil {
		return err
	}
	a.markOrganizationDutiful(ctx, logger, externalID, account.Number)
	_, err = a.Users.InformOrganizationBillingConfigured(
		ctx,
		&users.InformOrganizationBillingConfiguredRequest{
			ExternalID: externalID,
		},
	)
	if err != nil {
		logger.WithField("err", err).Errorln("Error calling InformOrganizationBillingConfigured")
	}

	// As the customer may have delayed setting up their account, we need to
	// upload any historic usage data since their trial period expired
	today := time.Now().UTC().Truncate(24 * time.Hour)
	// If the trial expires today we'll catch the usage in the uploader next time it's run
	if !resp.Organization.InTrialPeriod(today) {
		orgID := resp.Organization.ID
		trialExpiry := resp.Organization.TrialExpiresAt
		usageImportID, err := a.FetchAndUploadUsage(r.Context(), account, orgID, externalID, trialExpiry, today, zuora.BillCycleDay)
		if err != nil {
			return err
		}
		if usageImportID != "" {
			err = a.DB.InsertPostTrialInvoice(r.Context(), externalID, account.Number, string(usageImportID))
			if err != nil {
				return err
			}
		}
	}

	render.JSON(w, http.StatusCreated, account)
	return nil
}

func (a *API) deserializeCreateAccountRequest(logger logging.Interface, r *http.Request) (*createAccountRequest, error) {
	req := &createAccountRequest{}
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		logger.Errorf("Failed to deserialise %v: %v", r.Body, err)
		return nil, err
	}
	return req, nil
}

func (a *API) getOrganization(ctx context.Context, externalID string) (*users.GetOrganizationResponse, error) {
	resp, err := a.Users.GetOrganization(ctx, &users.GetOrganizationRequest{
		ID: &users.GetOrganizationRequest_ExternalID{ExternalID: externalID},
	})
	if err != nil {
		user.LogWith(ctx, logging.Global()).Errorf("Failed to get organization for %v: %v", externalID, err)
		return nil, err
	}
	return resp, nil
}

func (a *API) createZuoraAccount(ctx context.Context, logger logging.Interface, req *createAccountRequest, resp *users.GetOrganizationResponse) (*zuora.Account, error) {
	logger.Infof("Creating Zuora account for %v", req.WeaveID)
	account, err := a.Zuora.CreateAccount(
		ctx,
		req.WeaveID,
		req.BillToContact,
		req.Currency,
		req.PaymentMethodID,
		zuora.BillCycleDay,
		resp.Organization.TrialExpiresAt,
	)
	if err != nil {
		logger.Errorf("Failed to create Zuora account for %v: %v", req.WeaveID, err)
		return nil, err
	}
	return account, nil
}

// markOrganizationDutiful tells the user service that the organization is no longer delinquent.
func (a *API) markOrganizationDutiful(ctx context.Context, logger logging.Interface, externalID, zuoraAccountNumber string) {
	var err error
	_, err = a.Users.SetOrganizationZuoraAccount(ctx, &users.SetOrganizationZuoraAccountRequest{
		ExternalID: externalID, Number: zuoraAccountNumber,
	})
	if err != nil {
		logger.Errorf("Failed to set Zuora account for %v to %v", externalID, zuoraAccountNumber)
	}

	logger.Infof("Updating users service with billing account status for %v", externalID)
	_, err = a.Users.SetOrganizationFlag(ctx, &users.SetOrganizationFlagRequest{
		ExternalID: externalID, Flag: orgs.RefuseDataAccess, Value: false})
	if err != nil {
		logger.Errorf("Failed to update RefuseDataAccess for %v", externalID)
	}
	_, err = a.Users.SetOrganizationFlag(ctx, &users.SetOrganizationFlagRequest{
		ExternalID: externalID, Flag: orgs.RefuseDataUpload, Value: false})
	if err != nil {
		logger.Errorf("Failed to update RefuseDataUpload for %v", externalID)
	}
}

// FetchAndUploadUsage gets usage from the database and uploads it to Zuora.
func (a *API) FetchAndUploadUsage(ctx context.Context, account *zuora.Account, orgID, externalID string, trialExpiry, today time.Time, cycleDay int) (zuora.UsageUploadID, error) {
	aggs, err := a.getPostTrialChargeableUsage(ctx, orgID, trialExpiry, today)
	if err != nil {
		return "", err
	}
	if len(aggs) == 0 {
		return "", nil
	}
	usageImportID, err := a.uploadUsage(ctx, externalID, account, aggs, trialExpiry, today, cycleDay)
	if err != nil {
		return "", err
	}
	return usageImportID, nil
}

func (a *API) getPostTrialChargeableUsage(ctx context.Context, orgID string, trialExpiry, today time.Time) ([]db.Aggregate, error) {
	logger := user.LogWith(ctx, logging.Global())
	logger.Infof("Querying post-trial usage data for %v, %v -> %v", orgID, trialExpiry, today)
	aggs, err := a.DB.GetAggregates(ctx, orgID, trialExpiry, today)
	if err != nil {
		logger.Errorf("Failed to get aggregates from DB for %v from %v to %v: %v", orgID, trialExpiry, today, err)
		return nil, err
	}
	logger.Infof("Got %v aggregates from DB for %v from %v to %v: %v", len(aggs), orgID, trialExpiry, today, err)
	return aggs, nil
}

func (a *API) uploadUsage(ctx context.Context, externalID string, account *zuora.Account, aggs []db.Aggregate, trialExpiry, today time.Time, cycleDay int) (zuora.UsageUploadID, error) {
	logger := user.LogWith(ctx, logging.Global())
	// If the trial expired before today, then we need to upload the gap that would be missed
	aggs, err := zuora.FilterAggregatesForSubscription(ctx, a.Zuora, aggs, account)
	if err != nil {
		logger.Errorf("Failed to filter billable usage report for %v: %v", externalID, err)
		return "", err
	}
	subscriptionNumber := account.Subscription.SubscriptionNumber
	chargeNumber := account.Subscription.ChargeNumber
	report, err := zuora.ReportFromAggregates(
		a.Zuora.GetConfig(), aggs, account.PaymentProviderID, trialExpiry, today, subscriptionNumber, chargeNumber, cycleDay,
	)
	if err != nil {
		logger.Errorf("Failed to create usage report for %v/%v/%v: %v", externalID, subscriptionNumber, chargeNumber, err)
		return "", err
	}

	reader, err := report.ToZuoraFormat()
	if err != nil {
		logger.Errorf("Failed to format zuora report: %v", externalID)
		return "", err
	}

	logger.Infof("Uploading post-trial usage data for %v", externalID)
	importID, err := a.Zuora.UploadUsage(ctx, reader, fmt.Sprintf("p-%s", externalID))
	if err != nil {
		logger.Errorf("Failed to upload usage report for %v/%v/%v: %v", externalID, subscriptionNumber, chargeNumber, err)
		return "", err
	}

	return importID, nil
}

// CreateAccount creates an account on Zuora and uploads any pending usage data.
func (a *API) CreateAccount(w http.ResponseWriter, r *http.Request) {
	err := a.createAccount(w, r)
	if err != nil {
		renderError(w, r, err)
	}
}

// accountWithTrial is for api backwards compat.
type accountWithTrial struct {
	*zuora.Account
	User *organizationWithTrial `json:"user"`
}

// organizationWithTrial is for api backwards compat.
type organizationWithTrial struct {
	ExternalID string      `json:"id"`
	CreatedAt  time.Time   `json:"created"`
	Trial      trial.Trial `json:"trial"`
}

// GetAccount gets the account from Zuora.
func (a *API) GetAccount(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	resp, err := a.Users.GetOrganization(ctx, &users.GetOrganizationRequest{
		ID: &users.GetOrganizationRequest_ExternalID{ExternalID: mux.Vars(r)["id"]},
	})
	if err != nil {
		renderError(w, r, err)
		return
	}

	account, err := a.Zuora.GetAccount(ctx, resp.Organization.ZuoraAccountNumber)
	if err != nil {
		renderError(w, r, err)
		return
	}

	trial := trial.Info(resp.Organization.TrialExpiresAt, resp.Organization.CreatedAt, time.Now().UTC())

	render.JSON(w, http.StatusOK, accountWithTrial{
		Account: account,
		User: &organizationWithTrial{
			ExternalID: resp.Organization.ExternalID,
			CreatedAt:  resp.Organization.CreatedAt,
			Trial:      trial,
		},
	})
}

// UpdateAccount updates the account on Zuora.
func (a *API) UpdateAccount(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgExternalID := mux.Vars(r)["id"]

	req := &zuora.Account{}
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		renderError(w, r, err)
		return
	}
	resp, err := a.getOrganization(ctx, orgExternalID)
	if err != nil {
		renderError(w, r, err)
		return
	}
	org := resp.Organization
	account, err := a.Zuora.UpdateAccount(ctx, org.ZuoraAccountNumber, req)
	if err != nil {
		renderError(w, r, err)
		return
	}
	render.JSON(w, http.StatusOK, account)
}

// GetAccountTrial gets trial information about the account.
func (a *API) GetAccountTrial(w http.ResponseWriter, r *http.Request) {
	resp, err := a.Users.GetOrganization(r.Context(), &users.GetOrganizationRequest{
		ID: &users.GetOrganizationRequest_ExternalID{ExternalID: mux.Vars(r)["id"]},
	})
	if err != nil {
		renderError(w, r, err)
		return
	}
	trial := trial.Info(resp.Organization.TrialExpiresAt, resp.Organization.CreatedAt, time.Now().UTC())
	render.JSON(w, http.StatusOK, trial)
}

// status indicates the account's billing status. Values of this string must align with values in service-ui.
// See `renderAccountStatus` in
// https://github.com/weaveworks/service-ui/blob/master/client/src/pages/organization/billing/page-zuora.jsx
type status string

const (
	statusTrialActive          status = "trial"
	statusTrialExpired         status = "trial_expired"
	statusPaymentDue           status = "payment_due"
	statusPaymentError         status = "payment_error"
	statusSubscriptionInactive status = "subscription_inactive"
	statusActive               status = "active"
)

var billingStatusToAPI = map[grpc.BillingStatus]status{
	grpc.TRIAL_ACTIVE:          statusTrialActive,
	grpc.TRIAL_EXPIRED:         statusTrialExpired,
	grpc.PAYMENT_DUE:           statusPaymentDue,
	grpc.PAYMENT_ERROR:         statusPaymentError,
	grpc.SUBSCRIPTION_INACTIVE: statusSubscriptionInactive,
	grpc.ACTIVE:                statusActive,
}

type interim struct {
	Usage map[string]string `json:"usage"` // prices in all supported currencies.
	Start time.Time         `json:"start"`
	End   time.Time         `json:"end"`
}

type accountStatusResponse struct {
	Trial                 trial.Trial       `json:"trial"`
	BillingPeriodStart    string            `json:"billing_period_start"`
	BillingPeriodEnd      string            `json:"billing_period_end"`
	UsageToDate           map[string]string `json:"usage_to_date"` // prices in all supported currencies.
	UsagePerDay           map[string]int64  `json:"usage_per_day"` // in node-seconds; key is day in `YYYY-MM-DD`
	ActiveHosts           float64           `json:"active_hosts"`
	Status                paymentStatus     `json:"payment_status"`
	Interim               *interim          `json:"interim,omitempty"`
	EstimatedMonthlyUsage map[string]string `json:"estimated_monthly_usage"` // in all supported currencies.
	Currency              string            `json:"currency"`                // currency from Zuora's subscription, or default on.
}

type paymentStatus struct {
	Status      status `json:"status"`
	Description string `json:"description"`
	Action      string `json:"action"`
}

func monthBillDate(billCycleDay int, month time.Month, year int) time.Time {
	daysInMonth := timeutil.DaysIn(month, year)
	var billDay int
	if daysInMonth < billCycleDay {
		billDay = daysInMonth
	} else {
		billDay = billCycleDay
	}
	return time.Date(year, month, billDay, 0, 0, 0, 0, time.UTC)
}

func computeBillingPeriod(billCycleDay int, createdAt, trialEnd, reference time.Time) (time.Time, time.Time) {
	today := reference.Truncate(24 * time.Hour)
	billDateThisMonth := monthBillDate(billCycleDay, today.Month(), today.Year())

	var start, end time.Time
	// If the billing date is after today, the billing period starts in the previous month.
	// Otherwise, the billing period starts in the present month.
	if billDateThisMonth.After(today) {
		start = monthBillDate(billCycleDay, today.Month()-1, today.Year())
		end = billDateThisMonth
	} else {
		start = billDateThisMonth
		end = monthBillDate(billCycleDay, today.Month()+1, today.Year())
	}

	trialEndDay := trialEnd.Truncate(24 * time.Hour)
	if start.Before(createdAt) {
		start = createdAt.Truncate(24 * time.Hour)
	} else if start.Before(trialEnd) && end.After(trialEnd) {
		if reference.Before(trialEndDay) {
			end = trialEndDay
		} else {
			start = trialEndDay
		}
	}
	return start, end
}

// Read-through cache for Zuora's rates.
// WARNING: No eviction strategy, i.e. in case we change the rates, one has to manually restart billing-api.
var cachedRates zuora.RateMap

func (a *API) getDefaultUsageRateInfo(ctx context.Context) (int, map[string]float64, error) {
	if cachedRates == nil {
		// Load the rates from Zuora. This may take a few seconds.
		rates, err := a.Zuora.GetCurrentRates(ctx)
		if err != nil {
			return 0, make(map[string]float64), err
		}
		cachedRates = rates
	}
	return zuora.BillCycleDay, cachedRates[billing.UsageNodeSeconds], nil
}

// getBillingStatus returns a single overall summary of the user's account.
func getBillingStatus(ctx context.Context, trialInfo trial.Trial, acct *zuora.Account) paymentStatus {
	grpcStatus, description, action := api.GetBillingStatus(ctx, trialInfo, acct)
	// do some type conversion for backwards compatibility
	jsonStatus, ok := billingStatusToAPI[grpcStatus]
	if !ok {
		jsonStatus = status(strings.ToLower(grpcStatus.String()))
	}
	return paymentStatus{Status: jsonStatus, Description: description, Action: action}
}

// Introducing the contextKey alias addresses "should not use basic type untyped string as key in context.WithValue".
type contextKey string

// GetAccountStatus returns the account status as a JSON response.
func (a *API) GetAccountStatus(w http.ResponseWriter, r *http.Request) {
	orgID := mux.Vars(r)["id"]
	ctx := context.WithValue(r.Context(), contextKey("org_id"), orgID)
	resp, err := a.Users.GetOrganization(ctx, &users.GetOrganizationRequest{
		ID: &users.GetOrganizationRequest_ExternalID{ExternalID: orgID},
	})
	if err != nil {
		renderError(w, r, err)
		return
	}
	org := resp.Organization
	now := time.Now().UTC()

	billCycleDay, rates, err := a.getDefaultUsageRateInfo(ctx)
	if err != nil {
		renderError(w, r, err)
		return
	}

	currency := "" // Leave currency blank by default. It is only set if a subscription exists...
	zuoraAcct, err := a.Zuora.GetAccount(ctx, org.ZuoraAccountNumber)
	if err != nil && err != zuora.ErrNotFound && err != zuora.ErrInvalidAccountNumber {
		renderError(w, r, err)
		return
	}
	if zuoraAcct != nil {
		billCycleDay = zuoraAcct.BillCycleDay
		// ... in which case it can also be used to point to the current subscription's rate,
		// even though the subscription price should be identical to the default rates.
		currency = zuoraAcct.Subscription.Currency
		rates[currency] = zuoraAcct.Subscription.Price
	}

	start, end := computeBillingPeriod(billCycleDay, org.CreatedAt, org.TrialExpiresAt, now)
	// assumption: if a zuora account is present, the customer has been charged and therefore
	// interim usage is zero
	var interimPeriod *interim
	if zuoraAcct == nil && !org.InTrialPeriod(start) {
		interimAggs, err := a.DB.GetAggregates(ctx, org.ID, org.TrialExpiresAt, start)
		if err != nil {
			renderError(w, r, err)
			return
		}
		interimSum, _, _ := sumAndFilterAggregates(interimAggs)
		interimPeriod = &interim{
			Usage: calculatePrices(rates, float64(interimSum)),
			Start: org.TrialExpiresAt,
			End:   start,
		}
	}

	aggs, err := a.DB.GetAggregates(ctx, org.ID, start, end)
	if err != nil {
		renderError(w, r, err)
		return
	}

	sum, hourly, daily := sumAndFilterAggregates(aggs)

	// Look at the usage from a previous hourly bucket, picking a bucket to avoid finding a value that is too low.
	// We can't use the current hour because we don't aggregate until the end of an hour
	// We don't use the previous hour because sometimes usage arrives late, and won't be counted until the current hour is up
	bucket := time.Now().UTC().Truncate(time.Hour).Add(-2 * time.Hour)
	var activeHosts float64
	if bucketSeconds, ok := hourly[bucket]; ok {
		activeHosts = float64(bucketSeconds) / time.Hour.Seconds()
	}

	trial := trial.Info(org.TrialExpiresAt, org.CreatedAt, now)

	estFrom, estTo, estDays := computeEstimationPeriod(now, org.TrialExpiresAt)
	estAggs, err := a.DB.GetAggregates(ctx, org.ID, estFrom, estTo)
	if err != nil {
		renderError(w, r, err)
		return
	}

	render.JSON(w, http.StatusOK, accountStatusResponse{
		BillingPeriodStart:    start.Format(dayTimeLayout),
		BillingPeriodEnd:      end.Format(dayTimeLayout),
		Trial:                 trial,
		UsageToDate:           calculatePrices(rates, float64(sum)),
		ActiveHosts:           activeHosts,
		Status:                getBillingStatus(ctx, trial, zuoraAcct),
		UsagePerDay:           daily,
		Interim:               interimPeriod,
		Currency:              currency,
		EstimatedMonthlyUsage: estimatedMonthlyUsages(daily, start, estAggs, estDays, rates, now),
	})
}

func calculatePrices(rates map[string]float64, quantity float64) map[string]string {
	prices := make(map[string]string, len(rates))
	for currency, rate := range rates {
		prices[currency] = fmt.Sprintf("%.2f", rate*quantity)
	}
	return prices
}

func computeEstimationPeriod(now time.Time, trialExpiresAt time.Time) (time.Time, time.Time, int) {
	to := now.Truncate(24 * time.Hour)

	from := to.Add(-7 * 24 * time.Hour)
	if trialExpiresAt.Before(now) {
		// Do not cross into trial
		from = timeutil.MaxTime(from,
			trialExpiresAt.Add(24*time.Hour).Truncate(24*time.Hour))
	}

	days := int(to.Sub(from).Hours()) / 24
	return from, to, days
}

func estimatedMonthlyUsages(daily map[string]int64, start time.Time, estAggs []db.Aggregate, estDays int, rates map[string]float64, reference time.Time) map[string]string {
	today := reference.Truncate(24 * time.Hour)

	// Current billing period usage. This excludes usage of today.
	var sum int64
	todayfmt := today.Format(dayTimeLayout)
	for day, value := range daily {
		if day != todayfmt {
			sum += value
		}
	}
	dayCount := float64(today.Sub(start).Hours()) / 24
	daysInMonth := timeutil.EndOfMonth(start).Day()

	// Estimation over given past period
	var estSum int64
	for _, a := range estAggs {
		if a.AmountType == timeutil.NodeSeconds {
			estSum += a.AmountValue
		}
	}
	usage := float64(sum) + (float64(daysInMonth)-dayCount)*float64(estSum)/float64(estDays)
	return calculateAndFormatMonthlyUsages(usage, rates)
}

func calculateAndFormatMonthlyUsages(usage float64, rates map[string]float64) map[string]string {
	usages := make(map[string]string, len(rates))
	for currency, rate := range rates {
		usages[currency] = fmt.Sprintf("%0.f", math.Ceil(rate*usage))
	}
	return usages
}

func sumAndFilterAggregates(aggs []db.Aggregate) (int64, map[time.Time]int64, map[string]int64) {
	hourly := map[time.Time]int64{}
	daily := map[string]int64{}
	var sum int64
	for _, agg := range aggs {
		if agg.AmountType == billing.UsageNodeSeconds {
			sum += agg.AmountValue

			day := agg.BucketStart.Format(dayTimeLayout)
			daily[day] += agg.AmountValue
			// We sum hourly aggregates because we might have more than one for an hour
			hourly[agg.BucketStart] += agg.AmountValue
		}
	}
	return sum, hourly, daily
}
