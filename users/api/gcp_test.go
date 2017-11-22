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

var pendingSubscription_noConsumerID = partner.Subscription{
	Name:              "partnerSubscriptions/47426f1a-d744-4249-ae84-3f4fe194c107",
	ExternalAccountID: "E-F65F-C51C-67FE-D42F",
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

func Test_Org_BillingProviderGCP(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user := dbtest.GetUser(t, database)
	org, err := database.CreateOrganizationWithGCP(context.TODO(), user.ID, "acc", "cons", "sub/1", "standard")
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
	// Create an existing GCP instance
	user := dbtest.GetUser(t, database)
	accountID := "E-ACC"
	_, err := database.CreateOrganizationWithGCP(context.TODO(), user.ID, accountID, "cons", "sub/1", "standard")
	assert.NoError(t, err)

	// Mock API
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock_partner.NewMockAPI(ctrl)
	access := mock_partner.NewMockAccessor(ctrl)
	api := createAPI(client, access)

	w := httptest.NewRecorder()
	r := requestAs(t, user, "GET", "/api/users/gcp/subscribe", nil)

	client.EXPECT().
		ListSubscriptions(r.Context(), accountID).
		Return([]partner.Subscription{pendingSubscription_noConsumerID}, nil)

	access.EXPECT().
		RequestSubscription(r.Context(), r, pendingSubscription_noConsumerID.Name).
		Return(&pendingSubscription_noConsumerID, nil)

	_, err = api.GCPSubscribe(user, accountID, w, r)
	assert.Error(t, err)
	assert.Equal(t, "no consumer ID found", err.Error())
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

/*
func TestAPI_GCPSubscribe_resume(t *testing.T) {
	setup(t)
	defer cleanup(t)

	// Create an existing org
	user := dbtest.GetUser(t, database)
	accountID := "E-ACC"
	org, err := database.CreateOrganizationWithGCP(context.TODO(), user.ID, accountID, "cons", "sub/1", "standard")
	assert.NoError(t, err)

	w := httptest.NewRecorder()
	r := requestAs(t, user, "GET", "/api/users/gcp/subscribe", nil)

	err = app.GCPSubscribe(user, accountID, w, r)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
}
*/
