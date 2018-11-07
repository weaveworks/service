package marketing_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/weaveworks/service/users/marketing"
)

var today = time.Date(2018, time.February, 12, 0, 0, 0, 0, time.UTC)

func TestBatchUpsertOneProspectComingFromGCPShouldSet_Activated_on_GCP__c(t *testing.T) {
	mock := &marketing.MockGoketoClient{}
	client := marketing.NewMarketoClient(mock, "test")
	client.BatchUpsertProspect([]marketing.Prospect{
		{
			Email:             "foo@bar.com",
			SignupSource:      "gcp",
			ServiceCreatedAt:  today,
			ServiceLastAccess: today,
			CampaignID:        "123",
			LeadSource:        "baz",
		},
	})
	expectedReq := "{\"programName\":\"test\",\"lookupField\":\"email\",\"input\":[{\"email\":\"foo@bar.com\",\"Weave_Cloud_Signup_Source__c\":\"gcp\",\"Activated_on_GCP__c\":1,\"Weave_Cloud_Created_On__c\":\"2018-02-12\",\"Weave_Cloud_Last_Active__c\":\"2018-02-12\",\"leadSource\":\"baz\",\"salesforceCampaignID\":\"123\"}]}"
	assert.Equal(t, expectedReq, string(mock.LatestReq))
}

func TestBatchUpsertOneProspectNotComingFromGCPShouldNotUnset_Activated_on_GCP__c(t *testing.T) {
	mock := &marketing.MockGoketoClient{}
	client := marketing.NewMarketoClient(mock, "test")
	client.BatchUpsertProspect([]marketing.Prospect{
		{
			Email:             "foo@bar.com",
			SignupSource:      "earth",
			ServiceCreatedAt:  today,
			ServiceLastAccess: today,
			CampaignID:        "123",
			LeadSource:        "baz",
		},
	})
	expectedReq := "{\"programName\":\"test\",\"lookupField\":\"email\",\"input\":[{\"email\":\"foo@bar.com\",\"Weave_Cloud_Signup_Source__c\":\"earth\",\"Weave_Cloud_Created_On__c\":\"2018-02-12\",\"Weave_Cloud_Last_Active__c\":\"2018-02-12\",\"leadSource\":\"baz\",\"salesforceCampaignID\":\"123\"}]}"
	assert.Equal(t, expectedReq, string(mock.LatestReq))
}

func TestBatchUpsertManyProspects(t *testing.T) {
	mock := &marketing.MockGoketoClient{}
	client := marketing.NewMarketoClient(mock, "test")
	client.BatchUpsertProspect([]marketing.Prospect{
		{
			Email:             "foo@bar.com",
			SignupSource:      "gcp",
			ServiceCreatedAt:  today,
			ServiceLastAccess: today,
			CampaignID:        "123",
			LeadSource:        "baz",
		},
		{
			Email:             "donald@trump.com",
			SignupSource:      "whitehouse",
			ServiceCreatedAt:  today,
			ServiceLastAccess: today,
			CampaignID:        "US presidential 456",
			LeadSource:        "pretzel",
		},
	})
	expectedReq := "{\"programName\":\"test\",\"lookupField\":\"email\",\"input\":[{\"email\":\"foo@bar.com\",\"Weave_Cloud_Signup_Source__c\":\"gcp\",\"Activated_on_GCP__c\":1,\"Weave_Cloud_Created_On__c\":\"2018-02-12\",\"Weave_Cloud_Last_Active__c\":\"2018-02-12\",\"leadSource\":\"baz\",\"salesforceCampaignID\":\"123\"},{\"email\":\"donald@trump.com\",\"Weave_Cloud_Signup_Source__c\":\"whitehouse\",\"Weave_Cloud_Created_On__c\":\"2018-02-12\",\"Weave_Cloud_Last_Active__c\":\"2018-02-12\",\"leadSource\":\"pretzel\",\"salesforceCampaignID\":\"US presidential 456\"}]}"
	assert.Equal(t, expectedReq, string(mock.LatestReq))
}
