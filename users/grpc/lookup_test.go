package grpc_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jordan-wright/email"
	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/db"
	"github.com/weaveworks/service/users/db/dbtest"
	"github.com/weaveworks/service/users/emailer"
	"github.com/weaveworks/service/users/grpc"
	"github.com/weaveworks/service/users/sessions"
	"github.com/weaveworks/service/users/templates"
)

var (
	database     db.DB
	sessionStore sessions.Store
	server       users.UsersServer
	ctx          context.Context
	smtp         emailer.SMTPEmailer
)

func setup(t *testing.T) {
	database = dbtest.Setup(t)
	sessionStore = sessions.MustNewStore("Test-Session-Secret-Which-Is-64-Bytes-Long-aa1a166556cb719f531cd", false)
	templates := templates.MustNewEngine("../templates")
	smtp = emailer.SMTPEmailer{
		Templates:   templates,
		Domain:      "https://weave.test",
		FromAddress: "from@weave.test",
	}
	server = grpc.New(sessionStore, database, &smtp)
	ctx = context.Background()
}

func cleanup(t *testing.T) {
	dbtest.Cleanup(t, database)
}

func Test_SetOrganizationFlag(t *testing.T) {
	setup(t)
	defer cleanup(t)
	_, org := dbtest.GetOrg(t, database)

	assert.False(t, org.RefuseDataAccess)
	assert.False(t, org.RefuseDataUpload)

	_, err := server.SetOrganizationFlag(
		ctx, &users.SetOrganizationFlagRequest{
			ExternalID: org.ExternalID,
			Flag:       "RefuseDataAccess",
			Value:      true,
		})
	require.NoError(t, err)
	resp, _ := server.GetOrganization(ctx, &users.GetOrganizationRequest{
		ID: &users.GetOrganizationRequest_ExternalID{ExternalID: org.ExternalID},
	})
	require.True(t, resp.Organization.RefuseDataAccess)

	_, err = server.SetOrganizationFlag(
		ctx, &users.SetOrganizationFlagRequest{
			ExternalID: org.ExternalID,
			Flag:       "RefuseDataUpload",
			Value:      true,
		})
	require.NoError(t, err)
	resp, _ = server.GetOrganization(ctx, &users.GetOrganizationRequest{
		ID: &users.GetOrganizationRequest_ExternalID{ExternalID: org.ExternalID},
	})

	require.True(t, resp.Organization.RefuseDataUpload)

	_, err = server.SetOrganizationFlag(
		ctx, &users.SetOrganizationFlagRequest{
			ExternalID: org.ExternalID,
			Flag:       "",
			Value:      true,
		})
	require.Error(t, err)
}

func Test_SetOrganizationZuoraAccount(t *testing.T) {
	setup(t)
	defer cleanup(t)
	_, org := dbtest.GetOrg(t, database)

	// initial state
	assert.Nil(t, org.ZuoraAccountCreatedAt)
	assert.Empty(t, org.ZuoraAccountNumber)

	// set number, automatically sets createdAt
	_, err := server.SetOrganizationZuoraAccount(
		ctx, &users.SetOrganizationZuoraAccountRequest{
			ExternalID: org.ExternalID,
			Number:     "Wfirst-set",
		})
	assert.NoError(t, err)
	ts := time.Now()
	resp, _ := server.GetOrganization(ctx, &users.GetOrganizationRequest{
		ID: &users.GetOrganizationRequest_ExternalID{ExternalID: org.ExternalID},
	})
	assert.Equal(t, "Wfirst-set", resp.Organization.ZuoraAccountNumber)
	assert.True(t, resp.Organization.ZuoraAccountCreatedAt.Before(ts))

	// update number, also updates createdAt
	_, err = server.SetOrganizationZuoraAccount(
		ctx, &users.SetOrganizationZuoraAccountRequest{
			ExternalID: org.ExternalID,
			Number:     "Wupdate",
		})
	assert.NoError(t, err)
	resp, _ = server.GetOrganization(ctx, &users.GetOrganizationRequest{
		ID: &users.GetOrganizationRequest_ExternalID{ExternalID: org.ExternalID},
	})
	assert.Equal(t, "Wupdate", resp.Organization.ZuoraAccountNumber)
	assert.True(t, resp.Organization.ZuoraAccountCreatedAt.After(ts))

	// explicitly set createdAt
	createdAt := time.Now().Truncate(time.Second)
	_, err = server.SetOrganizationZuoraAccount(
		ctx, &users.SetOrganizationZuoraAccountRequest{
			ExternalID: org.ExternalID,
			Number:     "Wexplicit-date",
			CreatedAt:  &createdAt,
		})
	assert.NoError(t, err)
	resp, _ = server.GetOrganization(ctx, &users.GetOrganizationRequest{
		ID: &users.GetOrganizationRequest_ExternalID{ExternalID: org.ExternalID},
	})
	assert.Equal(t, "Wexplicit-date", resp.Organization.ZuoraAccountNumber)
	assert.True(t, resp.Organization.ZuoraAccountCreatedAt.Equal(createdAt))
}

func Test_NotifyTrialPendingExpiry(t *testing.T) {
	setup(t)
	defer cleanup(t)
	_, org := dbtest.GetOrg(t, database)

	var sent bool
	smtp.Sender = func(e *email.Email) error {
		sent = true
		return nil
	}

	// defaults
	assert.Nil(t, org.TrialExpiredNotifiedAt)
	assert.Nil(t, org.TrialPendingExpiryNotifiedAt)

	_, err := server.NotifyTrialPendingExpiry(ctx, &users.NotifyTrialPendingExpiryRequest{ExternalID: org.ExternalID})
	assert.NoError(t, err)
	assert.True(t, sent, "email has not been sent")

	// verify database changes
	newOrg, err := database.FindOrganizationByID(ctx, org.ExternalID)
	assert.NoError(t, err)
	assert.Nil(t, newOrg.TrialExpiredNotifiedAt)
	assert.NotNil(t, newOrg.TrialPendingExpiryNotifiedAt)
}

func Test_NotifyTrialExpired(t *testing.T) {
	setup(t)
	defer cleanup(t)
	_, org := dbtest.GetOrg(t, database)

	var sent bool
	smtp.Sender = func(e *email.Email) error {
		sent = true
		return nil
	}

	// defaults
	assert.Nil(t, org.TrialExpiredNotifiedAt)
	assert.Nil(t, org.TrialPendingExpiryNotifiedAt)

	_, err := server.NotifyTrialExpired(ctx, &users.NotifyTrialExpiredRequest{ExternalID: org.ExternalID})
	assert.NoError(t, err)
	assert.True(t, sent, "email has not been sent")

	// verify database changes
	newOrg, err := database.FindOrganizationByID(ctx, org.ExternalID)
	assert.NoError(t, err)
	assert.NotNil(t, newOrg.TrialExpiredNotifiedAt)
	assert.Nil(t, newOrg.TrialPendingExpiryNotifiedAt)
}

// Test_GetBillableOrganizations_NotExpired shows that we don't return
// organizations from GetBillableOrganizations that have yet to expire their
// trial period.
func Test_GetBillableOrganizations_NotExpired(t *testing.T) {
	setup(t)
	defer cleanup(t)
	org := makeBillingOrganization(t)

	now := org.TrialExpiresAt.Add(-5 * 24 * time.Hour)
	{
		resp, err := server.GetBillableOrganizations(ctx, &users.GetBillableOrganizationsRequest{Now: now})
		require.NoError(t, err)
		assert.Equal(t, []users.Organization(nil), resp.Organizations)
	}
	// Giving an organization a Zuora account doesn't make it billable.
	org = setZuoraAccount(t, org, "Wwhatever")
	{
		resp, err := server.GetBillableOrganizations(ctx, &users.GetBillableOrganizationsRequest{Now: now})
		require.NoError(t, err)
		assert.Equal(t, []users.Organization(nil), resp.Organizations)
	}
}

// Test_GetBillableOrganizations_Expired shows that we return organizations
// that have expired their trial period, because they are billable.
func Test_GetBillableOrganizations_Expired(t *testing.T) {
	setup(t)
	defer cleanup(t)
	org := makeBillingOrganization(t)
	org = setZuoraAccount(t, org, "Wwhatever")
	now := org.TrialExpiresAt.Add(5 * 24 * time.Hour)
	{
		resp, err := server.GetBillableOrganizations(ctx, &users.GetBillableOrganizationsRequest{Now: now})
		require.NoError(t, err)
		assert.Equal(t, []users.Organization{*org}, resp.Organizations)
	}
}

// Test_GetBillableOrganizations_SkipDelinquent shows that delinquent organizations
// are not billable.
func Test_GetBillableOrganizations_SkipDelinquent(t *testing.T) {
	setup(t)
	defer cleanup(t)

	org := makeBillingOrganization(t)
	now := org.TrialExpiresAt.Add(5 * 24 * time.Hour)
	// No Zuora account means not billable
	{
		resp, err := server.GetBillableOrganizations(ctx, &users.GetBillableOrganizationsRequest{Now: now})
		require.NoError(t, err)
		assert.Empty(t, resp.Organizations)
	}

	// Organization becomes billable if it has a Zuora account
	org = setZuoraAccount(t, org, "Wwhatever")
	{
		resp, err := server.GetBillableOrganizations(ctx, &users.GetBillableOrganizationsRequest{Now: now})
		require.NoError(t, err)
		assert.Equal(t, []users.Organization{*org}, resp.Organizations)
	}
}

// Test_GetBillableOrganizations_GCP shows that we return GCP organizations
// that have running subscriptions.
func Test_GetBillableOrganizations_GCP(t *testing.T) {
	setup(t)
	defer cleanup(t)
	org := makeGCPBillingOrganization(t)

	now := time.Now()
	{
		resp, err := server.GetBillableOrganizations(ctx, &users.GetBillableOrganizationsRequest{Now: now})
		require.NoError(t, err)
		assert.Equal(t, []users.Organization{*org}, resp.Organizations)
	}

	cancelGCPSubscription(t, org.GCP.AccountID)
	{
		resp, err := server.GetBillableOrganizations(ctx, &users.GetBillableOrganizationsRequest{Now: now})
		require.NoError(t, err)
		assert.Equal(t, []users.Organization(nil), resp.Organizations)
	}
}

// Test_GetTrialOrganizations_NotExpired shows GetTrialOrganizations returns
// organizations that have yet to reach the end of their trial period.
func Test_GetTrialOrganizations_NotExpired(t *testing.T) {
	setup(t)
	defer cleanup(t)
	org := makeBillingOrganization(t)

	now := org.TrialExpiresAt.Add(-5 * 24 * time.Hour)
	{
		resp, err := server.GetTrialOrganizations(ctx, &users.GetTrialOrganizationsRequest{Now: now})
		require.NoError(t, err)
		assert.Equal(t, []users.Organization{*org}, resp.Organizations)
	}
	org = setZuoraAccount(t, org, "Wwhatever")
	{
		resp, err := server.GetTrialOrganizations(ctx, &users.GetTrialOrganizationsRequest{Now: now})
		require.NoError(t, err)
		assert.Equal(t, []users.Organization{*org}, resp.Organizations)
	}
}

// Test_GetTrialOrganizations_NotExpired shows GetTrialOrganizations returns
// organizations that have yet to reach the end of their trial period.
func Test_GetTrialOrganizations_Expired(t *testing.T) {
	setup(t)
	defer cleanup(t)
	org := makeBillingOrganization(t)

	now := org.TrialExpiresAt.Add(5 * 24 * time.Hour)
	{
		resp, err := server.GetTrialOrganizations(ctx, &users.GetTrialOrganizationsRequest{Now: now})
		require.NoError(t, err)
		assert.Equal(t, []users.Organization(nil), resp.Organizations)
	}
	org = setZuoraAccount(t, org, "Wwhatever")
	{
		resp, err := server.GetTrialOrganizations(ctx, &users.GetTrialOrganizationsRequest{Now: now})
		require.NoError(t, err)
		assert.Equal(t, []users.Organization(nil), resp.Organizations)
	}
}

// Test_GetTrialOrganizations_GCP shows that
// GetDelinquentOrganizations does not return GCP organizations
func Test_GetTrialOrganizations_GCP(t *testing.T) {
	setup(t)
	defer cleanup(t)
	makeGCPBillingOrganization(t)

	now := time.Now()
	{
		resp, err := server.GetTrialOrganizations(ctx, &users.GetTrialOrganizationsRequest{Now: now})
		require.NoError(t, err)
		assert.Equal(t, []users.Organization(nil), resp.Organizations)
	}
}

// Test_GetDelinquentOrganizations shows that GetDelinquentOrganizations never
// returns organizations that are still in their trial period.
func Test_GetDelinquentOrganizations_NotExpired(t *testing.T) {
	setup(t)
	defer cleanup(t)
	org := makeBillingOrganization(t)

	now := org.TrialExpiresAt.Add(-5 * 24 * time.Hour)
	{
		resp, err := server.GetDelinquentOrganizations(ctx, &users.GetDelinquentOrganizationsRequest{Now: now})
		require.NoError(t, err)
		assert.Equal(t, []users.Organization(nil), resp.Organizations)
	}
	org = setZuoraAccount(t, org, "Wwhatever")
	{
		resp, err := server.GetDelinquentOrganizations(ctx, &users.GetDelinquentOrganizationsRequest{Now: now})
		require.NoError(t, err)
		assert.Equal(t, []users.Organization(nil), resp.Organizations)
	}
}

// Test_GetDelinquentOrganizations_Expired shows that
// GetDelinquentOrganizations only returns organizations that have no zuora
// account and aren't in their trial period.
func Test_GetDelinquentOrganizations_Expired(t *testing.T) {
	setup(t)
	defer cleanup(t)
	org := makeBillingOrganization(t)

	now := org.TrialExpiresAt.Add(5 * 24 * time.Hour)
	{
		resp, err := server.GetDelinquentOrganizations(ctx, &users.GetDelinquentOrganizationsRequest{Now: now})
		require.NoError(t, err)
		assert.Equal(t, []users.Organization{*org}, resp.Organizations)
	}
	org = setZuoraAccount(t, org, "Wwhatever")
	{
		resp, err := server.GetDelinquentOrganizations(ctx, &users.GetDelinquentOrganizationsRequest{Now: now})
		require.NoError(t, err)
		assert.Equal(t, []users.Organization(nil), resp.Organizations)
	}
}

// Test_GetDelinquentOrganizations_GCP shows that
// GetDelinquentOrganizations does not return GCP organizations
func Test_GetDelinquentOrganizations_GCP(t *testing.T) {
	setup(t)
	defer cleanup(t)
	makeGCPBillingOrganization(t)

	now := time.Now()
	{
		resp, err := server.GetDelinquentOrganizations(ctx, &users.GetDelinquentOrganizationsRequest{Now: now})
		require.NoError(t, err)
		assert.Equal(t, []users.Organization(nil), resp.Organizations)
	}
}

// setZuoraAccount sets a Zuora account.
func setZuoraAccount(t *testing.T, org *users.Organization, account string) *users.Organization {
	_, err := server.SetOrganizationZuoraAccount(
		ctx, &users.SetOrganizationZuoraAccountRequest{
			ExternalID: org.ExternalID,
			Number:     account,
		})
	require.NoError(t, err)
	newOrg, err := database.FindOrganizationByID(ctx, org.ExternalID)
	require.NoError(t, err)
	return newOrg
}

// makeBillingOrganization makes an organization that has billing enabled.
// Won't be necessary after we remove the billing feature flag.
func makeBillingOrganization(t *testing.T) *users.Organization {
	_, org := dbtest.GetOrg(t, database)
	err := database.AddFeatureFlag(ctx, org.ExternalID, users.BillingFeatureFlag)
	require.NoError(t, err)
	newOrg, err := database.FindOrganizationByID(ctx, org.ExternalID)
	require.NoError(t, err)
	return newOrg
}

func makeGCPBillingOrganization(t *testing.T) *users.Organization {
	user := dbtest.GetUser(t, database)
	org, err := database.CreateOrganizationWithGCP(
		context.Background(),
		user.ID,
		"E-97A7-79FC-AD2D-9D31",
		"project_number:123",
		"partnerSubscriptions/123",
		"standard",
	)
	require.NoError(t, err)

	// activate account
	err = database.UpdateGCP(context.Background(), org.GCP.AccountID, org.GCP.ConsumerID, org.GCP.SubscriptionName, org.GCP.SubscriptionLevel, true)
	require.NoError(t, err)

	newOrg, err := database.FindOrganizationByID(context.Background(), org.ExternalID)
	require.NoError(t, err)
	return newOrg
}

func cancelGCPSubscription(t *testing.T, gcpAccountID string) {
	err := database.UpdateGCP(context.Background(), gcpAccountID, "", "", "", true)
	require.NoError(t, err)
}

func Test_GetOrganization(t *testing.T) {
	setup(t)
	defer cleanup(t)

	_, org := dbtest.GetOrg(t, database)

	orgViaExternal, err := server.GetOrganization(ctx, &users.GetOrganizationRequest{
		ID: &users.GetOrganizationRequest_ExternalID{ExternalID: org.ExternalID},
	})

	if err != nil {
		t.Fatal(err)
	}

	orgViaInternal, err := server.GetOrganization(ctx, &users.GetOrganizationRequest{
		ID: &users.GetOrganizationRequest_InternalID{InternalID: org.ID},
	})

	if err != nil {
		t.Fatal(err)
	}

	if orgViaExternal.Organization.ID != orgViaInternal.Organization.ID {
		t.Fatalf(
			"Expected external and internal to match. External: %v; Internal: %v",
			orgViaExternal.Organization.ID,
			orgViaInternal.Organization.ID,
		)
	}

}

func Test_GetOrganization_NoIDSet(t *testing.T) {
	setup(t)
	defer cleanup(t)

	org, err := server.GetOrganization(ctx, &users.GetOrganizationRequest{})
	assert.Nil(t, org)
	assert.NotNil(t, err)
}
