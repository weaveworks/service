package api_test

import (
	"context"
	"encoding/json"
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

const externalAccountID = "E-F007-C51C-B33F-D34D"

var (
	pendingSubscription = partner.Subscription{
		Name:              "partnerSubscriptions/47426f1a-d744-4249-ae84-3f4fe194c107",
		ExternalAccountID: externalAccountID,
		Version:           "1508480169982224",
		Status:            "PENDING",
		SubscribedResources: []partner.SubscribedResource{{
			SubscriptionProvider: "weaveworks-public-cloudmarketplacepartner.googleapis.com",
			Resource:             "weave-cloud",
			Labels: map[string]string{
				"consumerId":  "project_number:123",
				"serviceName": "staging.google.weave.works",
				"weaveworks-public-cloudmarketplacepartner.googleapis.com/ServiceLevel": "standard",
			},
		}},
	}
	pendingSubscriptionNoConsumerID = partner.Subscription{
		Name:              "partnerSubscriptions/47426f1a-d744-4249-ae84-3f4fe194c107",
		ExternalAccountID: externalAccountID,
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
	org, err := database.CreateOrganizationWithGCP(context.TODO(), user.ID, externalAccountID)
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
	_, err := database.CreateOrganizationWithGCP(context.TODO(), user.ID, externalAccountID)
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
		ListSubscriptions(r.Context(), externalAccountID).
		Return([]partner.Subscription{pendingSubscriptionNoConsumerID}, nil)

	access.EXPECT().
		RequestSubscription(r.Context(), r, pendingSubscriptionNoConsumerID.Name).
		Return(&pendingSubscriptionNoConsumerID, nil)

	_, err = a.GCPSubscribe(user, externalAccountID, w, r)
	assert.Error(t, err)
	assert.Equal(t, api.ErrMissingConsumerID, err)
}

func TestAPI_GCPSubscribe(t *testing.T) {
	database = dbtest.Setup(t)
	defer dbtest.Cleanup(t, database)

	// Create an existing org
	user := dbtest.GetUser(t, database)

	// Mock API
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock_partner.NewMockAPI(ctrl)
	access := mock_partner.NewMockAccessor(ctrl)
	api := createAPI(client, access)

	w := httptest.NewRecorder()
	r := requestAs(t, user, "GET", "/api/users/gcp/subscribe", nil)

	client.EXPECT().
		ListSubscriptions(r.Context(), externalAccountID).
		Return([]partner.Subscription{pendingSubscription}, nil)
	access.EXPECT().
		RequestSubscription(r.Context(), r, pendingSubscription.Name).
		Return(&pendingSubscription, nil)
	client.EXPECT().
		ApproveSubscription(r.Context(), pendingSubscription.Name, gomock.Any()).
		Return(nil, nil)

	org, err := api.GCPSubscribe(user, externalAccountID, w, r)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)

	// Make sure account was activated and the subscription is running
	org, err = database.FindOrganizationByGCPAccountID(context.TODO(), externalAccountID)
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
	org, err := database.CreateOrganizationWithGCP(context.TODO(), user.ID, externalAccountID)
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
		ListSubscriptions(r.Context(), externalAccountID).
		Return([]partner.Subscription{pendingSubscription}, nil).Times(2)
	access.EXPECT().
		RequestSubscription(r.Context(), r, pendingSubscription.Name).
		Return(&pendingSubscription, nil).Times(2)
	client.EXPECT().
		ApproveSubscription(r.Context(), pendingSubscription.Name, gomock.Any()).
		Return(nil, nil)

	{ // Organization is created and GCP activated, with subscription running.
		org, err = a.GCPSubscribe(user, externalAccountID, w, r)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, w.Code)

		// Make sure account was activated and the subscription is running
		org, err = database.FindOrganizationByGCPAccountID(context.TODO(), externalAccountID)
		assert.NoError(t, err)
		assert.True(t, org.GCP.Activated)
		assert.EqualValues(t, partner.Active, org.GCP.SubscriptionStatus)
	}
	{ // It is not possible to resume signup of an already activated account
		org, err = a.GCPSubscribe(user, externalAccountID, w, r)
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
