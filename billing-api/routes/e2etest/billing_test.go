// Package e2etest does end to end testing by accessing the staging
// environment (sandbox) of Zuora.
package e2etest

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/golang/mock/gomock"

	"github.com/weaveworks/service/billing-api/db"
	"github.com/weaveworks/service/billing-api/db/mock_db"
	"github.com/weaveworks/service/billing-api/routes"
	"github.com/weaveworks/service/common/zuora"
	"github.com/weaveworks/service/common/zuora/mockzuora"
)

func TestMain(m *testing.M) {
	zuoraUsername := os.Getenv("ZUORA_USERNAME")
	zuoraPassword := os.Getenv("ZUORA_PASSWORD")
	zuoraSubscriptionPlanID := os.Getenv("ZUORA_SUBSCRIPTIONPLANID")

	if zuoraUsername == "" || zuoraPassword == "" || zuoraSubscriptionPlanID == "" {
		return
	}

	mockzuora.Config.Username = zuoraUsername
	mockzuora.Config.Password = zuoraPassword
	mockzuora.Config.SubscriptionPlanID = zuoraSubscriptionPlanID

	e := m.Run()

	mockzuora.Config = zuora.Config{}
	os.Exit(e)
}

func TestPaymentOnBillingDay(t *testing.T) {
	ctx := context.Background()
	externalID := externalID()
	now := time.Now().UTC()
	signup := now
	trialExpiry := now.Add(30 * 24 * time.Hour)
	z := zuoraClient()
	account, err := createZuoraAccount(ctx, z, externalID, trialExpiry, signup.Day())
	if err != nil {
		t.Errorf("Failed to create zuora account: %v", err)
	}
	defer z.DeleteAccount(ctx, account.ZuoraID)
	if account.BillCycleDay != signup.Day() {
		t.Errorf("Bill cycle day must be %v instead of %v", signup.Day(), account.BillCycleDay)
	}
	// no usage uploaded, since pre-trial
	paymentID, err := z.CreateInvoice(ctx, account.Number)
	if err == nil {
		t.Errorf("CreateInvoice should fail, there's no chargable usage, paymentID %v", paymentID)
	}
	if !z.NoChargeableUsage(err) {
		t.Errorf("CreateInvoice must return error code %v Error: %v", zuora.ErrRuleRestrictionInRestService, err)
	}
}

func TestPaymentInTrial(t *testing.T) {
	ctx := context.Background()
	externalID := externalID()
	now := time.Now().UTC()
	signup := now
	billCycleDay := 1
	if signup.Day() == billCycleDay {
		billCycleDay = signup.Add(-24 * time.Hour).Day()
	}
	trialExpiry := now.Add(15 * 24 * time.Hour)
	z := zuoraClient()
	account, err := createZuoraAccount(ctx, z, externalID, trialExpiry, signup.Day())
	if err != nil {
		t.Errorf("Failed to create zuora account: %v", err)
	}
	defer z.DeleteAccount(ctx, account.ZuoraID)
	// no usage uploaded, since pre-trial
	paymentID, err := z.CreateInvoice(ctx, account.Number)
	if err == nil {
		t.Errorf("CreateInvoice should fail, there's no chargable usage, paymentID %v", paymentID)
	}
	if !z.NoChargeableUsage(err) {
		t.Errorf("CreateInvoice must return error code %v. Error: %v", zuora.ErrRuleRestrictionInRestService, err)
	}
}

func TestPaymentAfterTrialSignupSameMonth(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ctx := context.Background()

	externalID := externalID()
	now := time.Now().UTC()
	// let's support the user signed up 25 days ago (not relevant)
	trialExpiry := now.Add(-5 * 24 * time.Hour) // 5 days ago
	// move the cycle day so that, the trial expires before the end of current period
	billCycleDay := trialExpiry.Add(-10 * 24 * time.Hour).Day() // 5 days before end of trial
	if !trialExpiry.Before(now) {
		t.Errorf("trialExpiry should be before now: %v, %v", trialExpiry, now)
	}
	z := zuoraClient()
	account, err := createZuoraAccount(ctx, z, externalID, trialExpiry, billCycleDay)
	if err != nil {
		t.Errorf("Failed to create zuora account: %v", err)
	}
	defer z.DeleteAccount(ctx, account.ZuoraID)
	orgID := externalID // it's mocked, so I make them equivalent (but they are not!)
	database := mock_db.NewMockDB(ctrl)
	database.EXPECT().
		GetAggregates(ctx, orgID, trialExpiry, now).
		Return([]db.Aggregate{
			{InstanceID: externalID, BucketStart: trialExpiry.Add(1 * 24 * time.Hour), AmountType: "node-seconds", AmountValue: 3600},
			{InstanceID: externalID, BucketStart: trialExpiry.Add(2 * 24 * time.Hour), AmountType: "node-seconds", AmountValue: 12000},
			{InstanceID: externalID, BucketStart: trialExpiry.Add(3 * 24 * time.Hour), AmountType: "node-seconds", AmountValue: 1728000},
		}, nil)
	a := &routes.API{Zuora: z, DB: database}
	importID, err := a.FetchAndUploadUsage(ctx, account, orgID, externalID, trialExpiry, now, billCycleDay)
	if err != nil {
		t.Errorf("Failed to fetch and/or upload usage: %v", err)
	}
	err = waitUntilUsageCompleted(ctx, z, 15*time.Second, importID)
	if err != nil {
		t.Error(err)
	}
	_, err = z.CreateInvoice(ctx, account.Number)
	if !z.NoChargeableUsage(err) {
		t.Errorf("Expected no chargable usage: %v", err)
	}
	invoices, err := z.GetInvoices(ctx, account.Number, "1", "40")
	if err != nil {
		t.Errorf("Failed to fetch invoices: %v", err)
	}
	if len(invoices) != 0 {
		t.Errorf("Expected exactly zero invoices, got: %v", len(invoices))
	}
}

func TestTrialExpiresPaymentNextMonth(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ctx := context.Background()
	externalID := externalID()
	now := time.Now().UTC()
	trialExpiry := now.Add(-10 * 24 * time.Hour)
	// moving billing cycle after trial expiry.
	// this means the zuora account is created in a billing period after the billing period
	// where the trial expired
	billCycleDay := now.Add(-4 * 24 * time.Hour).Day()
	if !trialExpiry.Before(now) {
		t.Errorf("trialExpiry should be before now: %v, %v", trialExpiry, now)
	}
	z := zuoraClient()
	account, err := createZuoraAccount(ctx, z, externalID, trialExpiry, billCycleDay)
	if err != nil {
		t.Errorf("Failed to create zuora account: %v", err)
	}
	defer z.DeleteAccount(ctx, account.ZuoraID)
	orgID := externalID // it's mocked, so I make them equivalent (but they are not!)
	var usageA, usageB, usageC int64 = 300, 12000, 1728000
	database := mock_db.NewMockDB(ctrl)
	database.EXPECT().
		GetAggregates(ctx, orgID, trialExpiry, now).
		Return([]db.Aggregate{
			{InstanceID: externalID, BucketStart: trialExpiry.Add(1 * 24 * time.Hour), AmountType: "node-seconds", AmountValue: usageA},
			{InstanceID: externalID, BucketStart: trialExpiry.Add(2 * 24 * time.Hour), AmountType: "node-seconds", AmountValue: usageB},
			{InstanceID: externalID, BucketStart: trialExpiry.Add(3 * 24 * time.Hour), AmountType: "node-seconds", AmountValue: usageC},
			// usage for two days ago, in current billing period, therefore not part of the invoice
			{InstanceID: externalID, BucketStart: now.Add(-2 * 24 * time.Hour), AmountType: "node-seconds", AmountValue: 1728000},
		}, nil)

	a := &routes.API{Zuora: z, DB: database}
	importID, err := a.FetchAndUploadUsage(ctx, account, orgID, externalID, trialExpiry, now, billCycleDay)
	if err != nil {
		t.Errorf("Failed to fetch and/or upload usage: %v", err)
	}
	err = waitUntilUsageCompleted(ctx, z, 15*time.Second, importID)
	if err != nil {
		t.Error(err)
	}
	_, err = z.CreateInvoice(ctx, account.Number)
	if err != nil {
		t.Errorf("Failed to generate invoice: %v", err)
	}
	invoices, err := z.GetInvoices(ctx, account.Number, "1", "40")
	if err != nil {
		t.Errorf("Failed to fetch invoices: %v", err)
	}
	if len(invoices) != 1 {
		t.Errorf("Expected exactly one invoice, got: %v", len(invoices))
	}
	unitPrice := unitPrices(ctx, z, t)["USD"]
	expectedAmount := float64(usageA+usageB+usageC) * unitPrice
	invoiceAmount := invoices[0].Amount
	if !routes.FloatEqual(invoiceAmount, routes.RoundHalfUp(expectedAmount)) {
		t.Errorf("Invoice amount different than expected %v != %v", invoiceAmount, expectedAmount)
	}
}

func TestTrialExpiresPaymentNextTwoMonth(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ctx := context.Background()
	externalID := externalID()
	now := time.Now().UTC()
	// at least two full month of trial-expired
	trialExpiry := now.Add(-70 * 24 * time.Hour)
	billCycleDay := now.Add(-5 * 24 * time.Hour).Day()
	if !trialExpiry.Before(now) {
		t.Errorf("trialExpiry should be before now: %v, %v", trialExpiry, now)
	}
	z := zuoraClient()
	account, err := createZuoraAccount(ctx, z, externalID, trialExpiry, billCycleDay)
	if err != nil {
		t.Errorf("Failed to create zuora account: %v", err)
	}
	defer z.DeleteAccount(ctx, account.ZuoraID)
	orgID := externalID // it's mocked, so I make them equivalent (but they are not!)
	var usageA, usageB, usageC, usageD int64 = 100000, 200000, 30000, 40000
	database := mock_db.NewMockDB(ctrl)
	database.EXPECT().
		GetAggregates(ctx, orgID, trialExpiry, now).
		Return([]db.Aggregate{
			{InstanceID: externalID, BucketStart: trialExpiry.Add(10 * 24 * time.Hour), AmountType: "node-seconds", AmountValue: usageA},
			{InstanceID: externalID, BucketStart: trialExpiry.Add(30 * 24 * time.Hour), AmountType: "node-seconds", AmountValue: usageB},
			{InstanceID: externalID, BucketStart: trialExpiry.Add(40 * 24 * time.Hour), AmountType: "node-seconds", AmountValue: usageC},
			{InstanceID: externalID, BucketStart: trialExpiry.Add(50 * 24 * time.Hour), AmountType: "node-seconds", AmountValue: usageD},
			// usage for yesterday, in current billing period, therefore not part of the invoice
			{InstanceID: externalID, BucketStart: now.Add(-1 * 24 * time.Hour), AmountType: "node-seconds", AmountValue: 1728000},
		}, nil)
	a := &routes.API{Zuora: z, DB: database}
	importID, err := a.FetchAndUploadUsage(ctx, account, orgID, externalID, trialExpiry, now, billCycleDay)
	if err != nil {
		t.Errorf("Failed to fetch and/or upload usage: %v", err)
	}
	err = waitUntilUsageCompleted(ctx, z, 15*time.Second, importID)
	if err != nil {
		t.Error(err)
	}
	_, err = z.CreateInvoice(ctx, account.Number)
	if err != nil {
		t.Errorf("Failed to generate invoice: %v", err)
	}
	invoices, err := z.GetInvoices(ctx, account.Number, "1", "40")
	if err != nil {
		t.Errorf("Failed to fetch invoices: %v", err)
	}
	if len(invoices) != 1 {
		t.Errorf("Expected exactly one invoice, got: %v", len(invoices))
	}
	unitPrice := unitPrices(ctx, z, t)["USD"]
	expectedAmount := float64(usageA+usageB+usageC+usageD) * unitPrice
	invoiceAmount := invoices[0].Amount
	if !routes.FloatEqual(invoiceAmount, routes.RoundHalfUp(expectedAmount)) {
		t.Errorf("Invoice amount different than expected %v != %v", invoiceAmount, expectedAmount)
	}
}

func externalID() string {
	now := time.Now().UTC()
	nanos := now.UnixNano()
	return fmt.Sprintf("billing-test-%v", nanos)
}

func zuoraClient() *zuora.Zuora {
	return zuora.New(mockzuora.Config, nil)
}

func unitPrices(ctx context.Context, z *zuora.Zuora, t *testing.T) map[string]float64 {
	rates, err := z.GetCurrentRates(ctx)
	if err != nil {
		t.Errorf("Failed to fetch price: %v", err)
	}
	prices := rates["node-seconds"]
	return prices
}

func createZuoraAccount(ctx context.Context, z *zuora.Zuora, externalID string, trialExpiry time.Time, BillCycleDay int) (*zuora.Account, error) {
	return z.CreateAccountWithCC(
		ctx,
		externalID,
		"USD",
		"Test",
		"Test",
		"GB",
		"",
		"",
		BillCycleDay,
		"MasterCard",
		"5555555555554444",
		12,
		2060,
		trialExpiry,
	)
}

func waitUntilUsageCompleted(ctx context.Context, z *zuora.Zuora, timeout time.Duration, importID string) error {
	startTime := time.Now().UTC()
	pollInterval := time.Duration(100 * time.Millisecond)
	for {
		importStatusResp, err := z.GetUsageImportStatus(ctx, z.URL("usage/%s/status", importID))
		if err != nil {
			return err
		}
		if importStatusResp.ImportStatus == zuora.Completed {
			return nil
		}
		if time.Now().UTC().Sub(startTime) >= timeout {
			return fmt.Errorf("waitUntilUsageCompleted timed out, status: %v, timeout: %v, importID %v", importStatusResp.ImportStatus, timeout, importID)
		}
		time.Sleep(pollInterval)
	}
}
