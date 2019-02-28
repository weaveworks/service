package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/weaveworks/service/common/gcp/procurement"
	"github.com/weaveworks/service/common/gcp/procurement/mock_procurement"
	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/api"
	"github.com/weaveworks/service/users/db"
	"github.com/weaveworks/service/users/db/dbtest"
	"github.com/weaveworks/service/users/sessions"
)

func Test_Org_BillingProviderGCP(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user := dbtest.GetUser(t, database)
	org, err := database.CreateOrganizationWithGCP(context.TODO(), user.ID, "E-BILL-ACC-ID", user.TrialExpiresAt())
	assert.NoError(t, err)

	w := httptest.NewRecorder()
	r := requestAs(t, user, "GET", "/api/users/org/"+org.ExternalID, nil)
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	body := map[string]interface{}{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "gcp", body["billingProvider"])
}

func TestAPI_GCPSubscribe2(t *testing.T) {
	database = dbtest.Setup(t)
	defer dbtest.Cleanup(t, database)
	ctrl, client, a := mockAPI(t)
	defer ctrl.Finish()

	testGCPSubscribe(t, database, a, client)
}

// TestAPI_GCPSubscribe_resumeInactivated verifies that you can resume an existing GCP account if
// he is inactivated and you cannot if he is activated.
func TestAPI_GCPSubscribe_resumeInactivated(t *testing.T) {
	database = dbtest.Setup(t)
	defer dbtest.Cleanup(t, database)
	ctrl, client, a := mockAPI(t)
	defer ctrl.Finish()

	// Create an existing org
	user := dbtest.GetUser(t, database)
	ent := makeEntitlement(procurement.ActivationRequested)
	accID := ent.AccountID()
	org, err := database.CreateOrganizationWithGCP(context.TODO(), user.ID, accID, user.TrialExpiresAt())
	assert.NoError(t, err)

	w := httptest.NewRecorder()
	r := requestAs(t, user, "GET", "/api/users/gcp/subscribe", nil)

	client.EXPECT().
		ListEntitlements(r.Context(), accID).
		Return([]procurement.Entitlement{ent}, nil).Times(2)
	client.EXPECT().
		ApproveAccount(r.Context(), ent.AccountID()).
		Return(nil)
	client.EXPECT().
		ApproveEntitlement(r.Context(), ent.Name).
		Return(nil)

	t.Run("resuming inactive GCP account", func(t *testing.T) {
		// Organization is created and GCP activated, with subscription running.
		org, err = a.GCPSubscribe(user, accID, w, r)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, w.Code)

		// Make sure account was activated and the subscription is running
		org, err = database.FindOrganizationByGCPExternalAccountID(context.TODO(), accID)
		assert.NoError(t, err)
		assert.True(t, org.GCP.Activated)
		assert.EqualValues(t, procurement.Active, org.GCP.SubscriptionStatus)
	})
	t.Run("resuming active GCP account fails", func(t *testing.T) {
		// It is not possible to resume signup of an already activated account
		org, err = a.GCPSubscribe(user, accID, w, r)
		assert.Error(t, err)
		assert.Equal(t, api.ErrAlreadyActivated, err)
	})
}

func TestAPI_GCPSSO_shouldGrantAccessAndBeIdempotent(t *testing.T) {
	database = dbtest.Setup(t)
	defer dbtest.Cleanup(t, database)
	ctrl, client, a := mockAPI(t)
	defer ctrl.Finish()
	// Setup a GCP instance by subscribing a new user (its owner):
	org, ent := testGCPSubscribe(t, database, a, client)

	// Create guest user. This user would authenticate using Google OAuth,
	// have their account created in Weave Cloud, and...
	guest := dbtest.GetUser(t, database)
	dbtest.AddGoogleLoginToUser(t, database, guest.ID)

	// ... by default do not have access to the instance previously created, but...
	hasAccess, err := database.UserIsMemberOf(context.TODO(), guest.ID, org.ExternalID)
	assert.NoError(t, err)
	assert.False(t, hasAccess)

	// ... eventually they'd go through the SSO flow which...
	r := requestAs(t, guest, "GET", fmt.Sprintf("/api/users/gcp/sso/login/%v", ent.AccountID()), nil)
	w := httptest.NewRecorder()
	orgInvitedTo, err := a.GCPSSOLogin(guest, ent.AccountID(), w, r)
	assert.NoError(t, err)
	assert.Equal(t, org.ID, orgInvitedTo.ID)

	// ... should grant them access to the instance previously created:
	hasAccess, err = database.UserIsMemberOf(context.TODO(), guest.ID, org.ExternalID)
	assert.NoError(t, err)
	assert.True(t, hasAccess)

	// If the guest user goes through SSO again, nothing should change, and they should still have access:
	r = requestAs(t, guest, "GET", fmt.Sprintf("/api/users/gcp/sso/login/%v", ent.AccountID()), nil)
	w = httptest.NewRecorder()
	orgInvitedTo, err = a.GCPSSOLogin(guest, ent.AccountID(), w, r)
	assert.NoError(t, err)
	assert.Equal(t, org.ID, orgInvitedTo.ID)
	hasAccess, err = database.UserIsMemberOf(context.TODO(), guest.ID, org.ExternalID)
	assert.NoError(t, err)
	assert.True(t, hasAccess)
}

func TestAPI_GCPSSO_shouldFailWithNotFoundOnNonExistentInstance(t *testing.T) {
	database = dbtest.Setup(t)
	defer dbtest.Cleanup(t, database)
	ctrl, _, a := mockAPI(t)
	defer ctrl.Finish()

	// Use an arbitrary IDs for a non-existent organization and subscription:
	orgExternalID := "Non-Existing Doom Flower"
	gcpExternalAccountID := "E-DEAD-BEEF-BADD-CAFE"

	// Create guest user. This user would authenticate using Google OAuth,
	// have their account created in Weave Cloud, and...
	guest := dbtest.GetUser(t, database)
	dbtest.AddGoogleLoginToUser(t, database, guest.ID)

	// ... by default do not have access to the instance, since non-existent, but...
	hasAccess, err := database.UserIsMemberOf(context.TODO(), guest.ID, orgExternalID)
	assert.NoError(t, err)
	assert.False(t, hasAccess)

	// ... eventually they'd go through the SSO flow which...
	r := requestAs(t, guest, "GET", fmt.Sprintf("/api/users/gcp/sso/login/%v", gcpExternalAccountID), nil)
	w := httptest.NewRecorder()
	orgInvitedTo, err := a.GCPSSOLogin(guest, gcpExternalAccountID, w, r)
	// ... fails with "not found", and...
	assert.Nil(t, orgInvitedTo)
	assert.Equal(t, users.ErrNotFound, err)
	// ... should still not grant access to the instance, since non-existent:
	hasAccess, err = database.UserIsMemberOf(context.TODO(), guest.ID, orgExternalID)
	assert.NoError(t, err)
	assert.False(t, hasAccess)
}

func mockAPI(t *testing.T) (*gomock.Controller, *mock_procurement.MockAPI, *api.API) {
	ctrl := gomock.NewController(t)
	client := mock_procurement.NewMockAPI(ctrl)
	a := createAPI(client)
	return ctrl, client, a
}

func testGCPSubscribe(t *testing.T, database db.DB, a *api.API, client *mock_procurement.MockAPI) (*users.Organization, procurement.Entitlement) {
	// Create an existing org
	user := dbtest.GetUser(t, database)

	w := httptest.NewRecorder()
	r := requestAs(t, user, "GET", "/api/users/gcp/subscribe", nil)

	ent := makeEntitlement(procurement.ActivationRequested)
	client.EXPECT().
		ListEntitlements(r.Context(), ent.AccountID()).
		Return([]procurement.Entitlement{ent}, nil)
	client.EXPECT().
		ApproveAccount(r.Context(), ent.AccountID()).
		Return(nil)
	client.EXPECT().
		ApproveEntitlement(r.Context(), ent.Name).
		Return(nil)

	org, err := a.GCPSubscribe(user, ent.AccountID(), w, r)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)

	// Make sure account was activated and the subscription is running
	org, err = database.FindOrganizationByGCPExternalAccountID(context.TODO(), ent.AccountID())
	assert.NoError(t, err)
	assert.True(t, org.GCP.Activated)
	assert.EqualValues(t, procurement.Active, org.GCP.SubscriptionStatus)

	return org, ent
}

func createAPI(client procurement.API) *api.API {
	sessionStore = sessions.MustNewStore("Test-Session-Secret-Which-Is-64-Bytes-Long-aa1a166556cb719f531cd", false, "")
	return api.New(
		false,
		nil,
		sessionStore,
		database,
		nil,
		nil,
		nil,
		nil,
		"",
		"",
		nil,
		make(map[string]struct{}),
		nil,
		client,
		"",
		"",
		"",
		"",
		"",
		nil,
		nil,
		"",
		nil,
	)
}
func makeEntitlement(state procurement.EntitlementState) procurement.Entitlement {
	return procurement.Entitlement{
		Name:             fmt.Sprintf("providers/weaveworks-dev/entitlements/%d", rand.Int63()),
		Account:          fmt.Sprintf("providers/weaveworks-dev/accounts/E-%d", rand.Int63()),
		Provider:         "weaveworks",
		Product:          "weave-cloud",
		Plan:             "standard",
		State:            state,
		NewPendingPlan:   "",
		UsageReportingID: "product_number:123",
	}
}
