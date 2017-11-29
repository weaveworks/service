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
	"github.com/weaveworks/service/users/api"
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
	org, err := database.CreateOrganizationWithGCP(context.TODO(), user.ID, "E-BILL-ACC-ID")
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

	// Create an existing GCP instance
	user := dbtest.GetUser(t, database)
	token := dbtest.AddGoogleLoginToUser(t, database, user.ID)
	_, err := database.CreateOrganizationWithGCP(context.TODO(), user.ID, pendingSubscriptionNoConsumerID.ExternalAccountID)
	assert.NoError(t, err)

	// Mock API
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock_partner.NewMockAPI(ctrl)
	access := mock_partner.NewMockAccessor(ctrl)
	a := createAPI(client, access)

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

	// Create an existing org
	user := dbtest.GetUser(t, database)
	token := dbtest.AddGoogleLoginToUser(t, database, user.ID)

	// Mock API
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock_partner.NewMockAPI(ctrl)
	access := mock_partner.NewMockAccessor(ctrl)
	a := createAPI(client, access)

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
}

// TestAPI_GCPSubscribe_resumeInactivated verifies that you can resume an existing GCP account if
// he is inactivated and you cannot if he is activated.
func TestAPI_GCPSubscribe_resumeInactivated(t *testing.T) {
	database = dbtest.Setup(t)
	defer dbtest.Cleanup(t, database)

	// Create an existing org
	user := dbtest.GetUser(t, database)
	token := dbtest.AddGoogleLoginToUser(t, database, user.ID)
	sub := makeSubscription()
	org, err := database.CreateOrganizationWithGCP(context.TODO(), user.ID, sub.ExternalAccountID)
	assert.NoError(t, err)

	// Mock API
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock_partner.NewMockAPI(ctrl)
	access := mock_partner.NewMockAccessor(ctrl)
	a := createAPI(client, access)

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

func createAPI(client partner.API, accessor partner.Accessor) *api.API {
	sessionStore = sessions.MustNewStore("Test-Session-Secret-Which-Is-64-Bytes-Long-aa1a166556cb719f531cd", false)
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
