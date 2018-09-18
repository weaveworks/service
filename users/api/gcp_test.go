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

	"github.com/weaveworks/service/common/gcp/partner"
	"github.com/weaveworks/service/common/gcp/partner/mock_partner"
	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/api"
	"github.com/weaveworks/service/users/db"
	"github.com/weaveworks/service/users/db/dbtest"
	"github.com/weaveworks/service/users/sessions"
)

var (
	pendingSubscriptionNoConsumerID = partner.Subscription{
		Name:              "partnerSubscriptions/47426f1a-d744-4249-ae84-3f4fe194c107",
		ExternalAccountID: "E-NO-CONSUMER-ID",
		Version:           "1508480169982224",
		Status:            "PENDING",
		SubscribedResources: []partner.SubscribedResource{{
			SubscriptionProvider: "weaveworks-public-cloudmarketplacepartner.googleapis.com",
			Resource:             "weave-cloud",
			Labels: map[string]string{
				"serviceName": "staging.google.weave.works",
				"weaveworks-public-cloudmarketplacepartner.googleapis.com/ServiceLevel": "standard",
			},
		}},
	}
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

func TestAPI_GCPSubscribe_missingConsumerID(t *testing.T) {
	database = dbtest.Setup(t)
	defer dbtest.Cleanup(t, database)
	ctrl, client, access, a := mockAPI(t)
	defer ctrl.Finish()

	// Create an existing GCP instance
	user := dbtest.GetUser(t, database)
	token := dbtest.AddGoogleLoginToUser(t, database, user.ID)
	_, err := database.CreateOrganizationWithGCP(context.TODO(), user.ID, pendingSubscriptionNoConsumerID.ExternalAccountID, user.TrialExpiresAt())
	assert.NoError(t, err)

	w := httptest.NewRecorder()
	r := requestAs(t, user, "GET", "/api/users/gcp/subscribe", nil)

	client.EXPECT().
		ListSubscriptions(r.Context(), pendingSubscriptionNoConsumerID.ExternalAccountID).
		Return([]partner.Subscription{pendingSubscriptionNoConsumerID}, nil)

	access.EXPECT().
		RequestSubscription(r.Context(), token, pendingSubscriptionNoConsumerID.Name).
		Return(&pendingSubscriptionNoConsumerID, nil)

	_, err = a.GCPSubscribe(user, pendingSubscriptionNoConsumerID.ExternalAccountID, w, r)
	assert.Error(t, err)
	assert.Equal(t, api.ErrMissingConsumerID, err)
}

func TestAPI_GCPSubscribe(t *testing.T) {
	database = dbtest.Setup(t)
	defer dbtest.Cleanup(t, database)
	ctrl, client, access, a := mockAPI(t)
	defer ctrl.Finish()

	gcpSubscribe(t, database, a, client, access)
}

// TestAPI_GCPSubscribe_resumeInactivated verifies that you can resume an existing GCP account if
// he is inactivated and you cannot if he is activated.
func TestAPI_GCPSubscribe_resumeInactivated(t *testing.T) {
	database = dbtest.Setup(t)
	defer dbtest.Cleanup(t, database)
	ctrl, client, access, a := mockAPI(t)
	defer ctrl.Finish()

	// Create an existing org
	user := dbtest.GetUser(t, database)
	token := dbtest.AddGoogleLoginToUser(t, database, user.ID)
	sub := makeSubscription()
	org, err := database.CreateOrganizationWithGCP(context.TODO(), user.ID, sub.ExternalAccountID, user.TrialExpiresAt())
	assert.NoError(t, err)

	w := httptest.NewRecorder()
	r := requestAs(t, user, "GET", "/api/users/gcp/subscribe", nil)

	client.EXPECT().
		ListSubscriptions(r.Context(), sub.ExternalAccountID).
		Return([]partner.Subscription{sub}, nil).Times(2)
	access.EXPECT().
		RequestSubscription(r.Context(), token, sub.Name).
		Return(&sub, nil).Times(2)
	client.EXPECT().
		ApproveSubscription(r.Context(), sub.Name, gomock.Any()).
		Return(nil, nil)

	{ // Organization is created and GCP activated, with subscription running.
		org, err = a.GCPSubscribe(user, sub.ExternalAccountID, w, r)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, w.Code)

		// Make sure account was activated and the subscription is running
		org, err = database.FindOrganizationByGCPExternalAccountID(context.TODO(), sub.ExternalAccountID)
		assert.NoError(t, err)
		assert.True(t, org.GCP.Activated)
		assert.EqualValues(t, partner.Active, org.GCP.SubscriptionStatus)
	}
	{ // It is not possible to resume signup of an already activated account
		org, err = a.GCPSubscribe(user, sub.ExternalAccountID, w, r)
		assert.Error(t, err)
		assert.Equal(t, api.ErrAlreadyActivated, err)
	}
}

func TestAPI_GCPSSO_shouldGrantAccessAndBeIdempotent(t *testing.T) {
	database = dbtest.Setup(t)
	defer dbtest.Cleanup(t, database)
	ctrl, client, access, a := mockAPI(t)
	defer ctrl.Finish()
	// Setup a GCP instance by subscribing a new user (its owner):
	org, sub := gcpSubscribe(t, database, a, client, access)

	// Create guest user. This user would authenticate using Google OAuth,
	// have their account created in Weave Cloud, and...
	guest := dbtest.GetUser(t, database)
	dbtest.AddGoogleLoginToUser(t, database, guest.ID)

	// ... by default do not have access to the instance previously created, but...
	hasAccess, err := database.UserIsMemberOf(context.TODO(), guest.ID, org.ExternalID)
	assert.NoError(t, err)
	assert.False(t, hasAccess)

	// ... eventually they'd go through the SSO flow which...
	r := requestAs(t, guest, "GET", fmt.Sprintf("/api/users/gcp/sso/login/%v", sub.ExternalAccountID), nil)
	w := httptest.NewRecorder()
	orgInvitedTo, err := a.GCPSSOLogin(guest, sub.ExternalAccountID, w, r)
	assert.NoError(t, err)
	assert.Equal(t, org.ID, orgInvitedTo.ID)

	// ... should grant them access to the instance previously created:
	hasAccess, err = database.UserIsMemberOf(context.TODO(), guest.ID, org.ExternalID)
	assert.NoError(t, err)
	assert.True(t, hasAccess)

	// If the guest user goes through SSO again, nothing should change, and they should still have access:
	r = requestAs(t, guest, "GET", fmt.Sprintf("/api/users/gcp/sso/login/%v", sub.ExternalAccountID), nil)
	w = httptest.NewRecorder()
	orgInvitedTo, err = a.GCPSSOLogin(guest, sub.ExternalAccountID, w, r)
	assert.NoError(t, err)
	assert.Equal(t, org.ID, orgInvitedTo.ID)
	hasAccess, err = database.UserIsMemberOf(context.TODO(), guest.ID, org.ExternalID)
	assert.NoError(t, err)
	assert.True(t, hasAccess)
}

func TestAPI_GCPSSO_shouldFailWithNotFoundOnNonExistentInstance(t *testing.T) {
	database = dbtest.Setup(t)
	defer dbtest.Cleanup(t, database)
	ctrl, _, _, a := mockAPI(t)
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

func mockAPI(t *testing.T) (*gomock.Controller, *mock_partner.MockAPI, *mock_partner.MockAccessor, *api.API) {
	ctrl := gomock.NewController(t)
	client := mock_partner.NewMockAPI(ctrl)
	access := mock_partner.NewMockAccessor(ctrl)
	a := createAPI(client, access)
	return ctrl, client, access, a
}

func gcpSubscribe(t *testing.T, database db.DB, a *api.API, client *mock_partner.MockAPI, access *mock_partner.MockAccessor) (*users.Organization, partner.Subscription) {
	// Create an existing org
	user := dbtest.GetUser(t, database)
	token := dbtest.AddGoogleLoginToUser(t, database, user.ID)

	w := httptest.NewRecorder()
	r := requestAs(t, user, "GET", "/api/users/gcp/subscribe", nil)

	sub := makeSubscription()
	client.EXPECT().
		ListSubscriptions(r.Context(), sub.ExternalAccountID).
		Return([]partner.Subscription{sub}, nil)
	access.EXPECT().
		RequestSubscription(r.Context(), token, sub.Name).
		Return(&sub, nil)
	client.EXPECT().
		ApproveSubscription(r.Context(), sub.Name, gomock.Any()).
		Return(nil, nil)

	org, err := a.GCPSubscribe(user, sub.ExternalAccountID, w, r)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)

	// Make sure account was activated and the subscription is running
	org, err = database.FindOrganizationByGCPExternalAccountID(context.TODO(), sub.ExternalAccountID)
	assert.NoError(t, err)
	assert.True(t, org.GCP.Activated)
	assert.EqualValues(t, partner.Active, org.GCP.SubscriptionStatus)

	return org, sub
}

func createAPI(client partner.API, accessor partner.Accessor) *api.API {
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
		accessor,
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
func makeSubscription() partner.Subscription {
	return partner.Subscription{
		Name:              fmt.Sprintf("partnerSubscriptions/%d", rand.Int63()),
		ExternalAccountID: fmt.Sprintf("E-%d", rand.Int63()),
		Version:           "1508480169982224",
		Status:            "PENDING",
		SubscribedResources: []partner.SubscribedResource{{
			SubscriptionProvider: "weaveworks-public-cloudmarketplacepartner.googleapis.com",
			Resource:             "weave-cloud",
			Labels: map[string]string{
				"consumerId":  fmt.Sprintf("project_number:%d", rand.Int63()),
				"serviceName": "staging.google.weave.works",
				"weaveworks-public-cloudmarketplacepartner.googleapis.com/ServiceLevel": "standard",
			},
		}},
	}

}
