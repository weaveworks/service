package routes_test

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/weaveworks/service/billing-api/db"
	"github.com/weaveworks/service/billing-api/db/mock_db"
	"github.com/weaveworks/service/billing-api/routes"
	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/mock_users"
)

func TestExportAsCSV(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	usersClient := mock_users.NewMockUsersClient(ctrl)
	usersClient.EXPECT().
		GetSummary(gomock.Any(), &users.Empty{}).
		Return(&users.Summary{
			Entries: []*users.SummaryEntry{
				{
					TeamExternalID:               "team-foo",
					TeamName:                     "Foo",
					OrgID:                        "1",
					OrgExternalID:                "foo-bar-1337",
					OrgName:                      "Foo",
					OrgCreatedAt:                 time.Date(2018, 04, 04, 23, 59, 59, 999999999, time.UTC),
					Emails:                       []string{"marc@weave.works", "roli@weave.works"},
					Platform:                     "kubernetes",
					Environment:                  "gke",
					TrialExpiresAt:               time.Date(2018, 05, 04, 23, 59, 59, 999999999, time.UTC),
					BillingEnabled:               true,
					RefuseDataAccess:             false,
					RefuseDataUpload:             false,
					GCPAccountExternalID:         "E-0000-0000-0000-0000",
					GCPAccountCreatedAt:          time.Date(2018, 04, 05, 00, 00, 00, 999999999, time.UTC),
					GCPAccountSubscriptionLevel:  "standard",
					GCPAccountSubscriptionStatus: "ACTIVE",
				},
				{
					TeamExternalID:               "team-foo",
					TeamName:                     "Foo",
					OrgID:                        "2",
					OrgExternalID:                "baz-baz-42",
					OrgName:                      "Baz",
					OrgCreatedAt:                 time.Date(2018, 04, 04, 0, 0, 0, 0, time.UTC),
					Emails:                       []string{"marc@weave.works"},
					Platform:                     "docker",
					Environment:                  "linux",
					FirstSeenConnectedAt:         addressOf(time.Date(2018, 04, 05, 0, 0, 0, 0, time.UTC)),
					TrialExpiresAt:               time.Date(2018, 05, 04, 11, 23, 00, 000000000, time.UTC),
					TrialPendingExpiryNotifiedAt: addressOf(time.Date(2018, 05, 04, 21, 59, 59, 999999999, time.UTC)),
					TrialExpiredNotifiedAt:       addressOf(time.Date(2018, 05, 04, 22, 59, 59, 999999999, time.UTC)),
					RefuseDataAccess:             true,
					RefuseDataUpload:             true,
					ZuoraAccountNumber:           "W0000000000000000000000000000000",
					ZuoraAccountCreatedAt:        addressOf(time.Date(2018, 04, 04, 1, 0, 0, 0, time.UTC)),
				},
			},
		}, nil)

	database := mock_db.NewMockDB(ctrl)
	database.EXPECT().
		GetMonthSums(gomock.Any(), []string{"1", "2"}, time.Date(2017, 10, 01, 0, 0, 0, 0, time.UTC), time.Date(2018, 04, 05, 0, 0, 0, 0, time.UTC)).
		Return(
			map[string][]db.Aggregate{
				"2": {
					{InstanceID: "baz-baz-42", BucketStart: time.Date(2018, 04, 04, 0, 0, 0, 0, time.UTC), AmountType: "container-seconds", AmountValue: 3600},
					{InstanceID: "baz-baz-42", BucketStart: time.Date(2018, 04, 04, 0, 0, 0, 0, time.UTC), AmountType: "node-seconds", AmountValue: 12000},
					{InstanceID: "baz-baz-42", BucketStart: time.Date(2018, 04, 04, 0, 0, 0, 0, time.UTC), AmountType: "samples", AmountValue: 1728000},
				},
			},
			nil)

	api := &routes.API{Users: usersClient, DB: database}

	router := mux.NewRouter()
	api.RegisterRoutes(router)
	rep := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/admin/billing.csv?from=2017-10-01&to=2018-04-04", nil)
	assert.NoError(t, err)
	router.ServeHTTP(rep, req)

	assert.Equal(t, http.StatusOK, rep.Code)

	body, err := ioutil.ReadAll(rep.Body)
	assert.NoError(t, err, "failed reading response body")

	assert.Equal(t,
		`TeamExternalID,TeamName,OrgID,OrgExternalID,OrgName,OrgCreatedAt,Emails,FirstSeenConnectedAt,Platform,Environment,TrialExpiresAt,TrialPendingExpiryNotifiedAt,TrialExpiredNotifiedAt,BillingEnabled,RefuseDataAccess,RefuseDataUpload,ZuoraAccountNumber,ZuoraAccountCreatedAt,GCPAccountExternalID,GCPAccountCreatedAt,GCPAccountSubscriptionLevel,GCPAccountSubscriptionStatus,container-seconds in April,node-seconds in April,samples in April,container-seconds in March,node-seconds in March,samples in March,container-seconds in February,node-seconds in February,samples in February,container-seconds in January,node-seconds in January,samples in January,container-seconds in December,node-seconds in December,samples in December,container-seconds in November,node-seconds in November,samples in November,container-seconds in October,node-seconds in October,samples in October
team-foo,Foo,1,foo-bar-1337,Foo,2018-04-04T23:59:59Z,marc@weave.works ; roli@weave.works,,kubernetes,gke,2018-05-04T23:59:59Z,,,true,false,false,,,E-0000-0000-0000-0000,2018-04-05T00:00:00Z,standard,ACTIVE,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0
team-foo,Foo,2,baz-baz-42,Baz,2018-04-04T00:00:00Z,marc@weave.works,2018-04-05T00:00:00Z,docker,linux,2018-05-04T11:23:00Z,2018-05-04T21:59:59Z,2018-05-04T22:59:59Z,false,true,true,W0000000000000000000000000000000,2018-04-04T01:00:00Z,,,,,3600,12000,1728000,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0
`,
		string(body))
}

// Because Go sucks, and fails with "cannot take the address of" if you directly do &time.Date(...).
func addressOf(t time.Time) *time.Time {
	return &t
}
