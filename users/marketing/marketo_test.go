package marketing_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/weaveworks/service/users/marketing"
)

type mockGoketoClient struct {
	LatestReq []byte
}

// RefreshToken does nothing.
func (m mockGoketoClient) RefreshToken() error {
	return nil
}

// Post returns a fake successful response.
func (m *mockGoketoClient) Post(resource string, data []byte) ([]byte, error) {
	m.LatestReq = data
	return []byte("{\"requestId\": \"foo\",\"result\": [{\"id\": 1337,\"status\": \"created\"}],\"success\": true}"), nil
}

var today = time.Date(2018, time.February, 12, 0, 0, 0, 0, time.UTC)

func TestBatchUpsertOneProspect(t *testing.T) {
	mock := &mockGoketoClient{}
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
	expectedReq := "{\"programName\":\"test\",\"lookupField\":\"email\",\"input\":[{\"email\":\"foo@bar.com\",\"Weave_Cloud_Signup_Source__c\":\"gcp\",\"Activated_on_GCP__c\":1,\"Weave_Cloud_Created_On__c\":\"2018-02-12\",\"Weave_Cloud_Last_Active__c\":\"2018-02-12\",\"Lead_Source__c\":\"baz\",\"salesforceCampaignID\":\"123\"}]}"
	assert.Equal(t, expectedReq, string(mock.LatestReq))
}

func TestBatchUpsertManyProspects(t *testing.T) {
	mock := &mockGoketoClient{}
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
	expectedReq := "{\"programName\":\"test\",\"lookupField\":\"email\",\"input\":[{\"email\":\"foo@bar.com\",\"Weave_Cloud_Signup_Source__c\":\"gcp\",\"Activated_on_GCP__c\":1,\"Weave_Cloud_Created_On__c\":\"2018-02-12\",\"Weave_Cloud_Last_Active__c\":\"2018-02-12\",\"Lead_Source__c\":\"baz\",\"salesforceCampaignID\":\"123\"},{\"email\":\"donald@trump.com\",\"Weave_Cloud_Signup_Source__c\":\"whitehouse\",\"Activated_on_GCP__c\":0,\"Weave_Cloud_Created_On__c\":\"2018-02-12\",\"Weave_Cloud_Last_Active__c\":\"2018-02-12\",\"Lead_Source__c\":\"pretzel\",\"salesforceCampaignID\":\"US presidential 456\"}]}"
	assert.Equal(t, expectedReq, string(mock.LatestReq))
}
