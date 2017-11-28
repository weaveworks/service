package job_test

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"io"
	"io/ioutil"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"google.golang.org/api/servicecontrol/v1"

	"github.com/weaveworks/common/instrument"
	"github.com/weaveworks/service/billing-api/db"
	"github.com/weaveworks/service/billing-api/db/dbtest"
	"github.com/weaveworks/service/billing-uploader/job"
	"github.com/weaveworks/service/billing-uploader/job/usage"
	"github.com/weaveworks/service/common/zuora"
	"github.com/weaveworks/service/common/zuora/mockzuora"
	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/mock_users"
)

type stubZuoraClient struct {
	mockzuora.StubClient

	err         error
	uploadUsage io.Reader
}

func (z *stubZuoraClient) GetAccount(ctx context.Context, weaveUserID string) (*zuora.Account, error) {
	return &zuora.Account{
		PaymentProviderID: "P" + weaveUserID,
		Subscription: &zuora.AccountSubscription{
			SubscriptionNumber: "S" + weaveUserID,
			ChargeNumber:       "C" + weaveUserID,
		},
	}, nil
}

func (z *stubZuoraClient) UploadUsage(ctx context.Context, r io.Reader) (string, error) {
	z.uploadUsage = r
	return "", z.err
}

// stubControlClient implements control.API
type stubControlClient struct {
	operations []*servicecontrol.Operation
}

func (c *stubControlClient) Report(ctx context.Context, operations []*servicecontrol.Operation) error {
	c.operations = operations
	return nil
}

func (c *stubControlClient) OperationID(name string) string {
	return "opid"
}

var (
	start         = time.Now().Truncate(24 * time.Hour).Add(-1 * 24 * time.Hour)
	expires       = start.Add(-2 * 24 * time.Hour)
	organizations = []users.Organization{
		// Zuora accounts
		{
			ID:                 "",
			ZuoraAccountNumber: "Wfoo",
			ExternalID:         "skip-empty_id",
		},
		{
			ID:                 "100",
			ZuoraAccountNumber: "Wboo",
			ExternalID:         "partial-max_aggregates_id",
		},
		{
			ID:                 "101",
			ZuoraAccountNumber: "Wzoo",
			ExternalID:         "partial-trial_expires_at",
			TrialExpiresAt:     expires,
		},
		// GCP accounts
		{
			ID:         "200",
			ExternalID: "yes",
			GCP: &users.GoogleCloudPlatform{
				ExternalAccountID:  "F-EEE",
				ConsumerID:         "project_number:123",
				Activated:          true,
				SubscriptionName:   "partnerSubscriptions/123",
				SubscriptionLevel:  "standard",
				SubscriptionStatus: "ACTIVE",
			},
		},
		{
			ID:         "201",
			ExternalID: "skip-inactivated",
			GCP: &users.GoogleCloudPlatform{
				ExternalAccountID:  "F-EEE",
				ConsumerID:         "project_number:123",
				Activated:          false,
				SubscriptionName:   "partnerSubscriptions/123",
				SubscriptionLevel:  "standard",
				SubscriptionStatus: "ACTIVE",
			},
		},
		{
			ID:         "202",
			ExternalID: "skip-inactive_subscription",
			GCP: &users.GoogleCloudPlatform{
				ExternalAccountID:  "F-EEE",
				ConsumerID:         "project_number:123",
				Activated:          true,
				SubscriptionStatus: "",
			},
		},
	}
	aggregates = []db.Aggregate{
		// Zuora
		{ // ID==1; skip: <=max_aggregate_id
			BucketStart: start,
			InstanceID:  "100",
			AmountType:  "node-seconds",
			AmountValue: 1,
		},
		{ // ID==2; pick
			BucketStart: start.Add(1 * time.Hour),
			InstanceID:  "100",
			AmountType:  "node-seconds",
			AmountValue: 2,
		},
		{ // ID==3; skip: <trial_expires_at
			BucketStart: expires.Add(-1 * time.Minute),
			InstanceID:  "101",
			AmountType:  "node-seconds",
			AmountValue: 3,
		},
		{ // ID==4; pick
			BucketStart: expires.Add(1 * time.Minute),
			InstanceID:  "101",
			AmountType:  "node-seconds",
			AmountValue: 4,
		},

		// GCP
		{ // ID==5
			BucketStart: start,
			InstanceID:  "200",
			AmountType:  "node-seconds",
			AmountValue: 10800,
		},
		{ // ID==6
			BucketStart: start.Add(-1 * time.Hour),
			InstanceID:  "200",
			AmountType:  "node-seconds",
			AmountValue: 1,
		},
		{ // ID==7; skip
			BucketStart: start,
			InstanceID:  "201",
			AmountType:  "node-seconds",
			AmountValue: 2,
		},
	}
)

func TestJobUpload_Do(t *testing.T) {
	d := dbtest.Setup(t)
	defer dbtest.Cleanup(t, d)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()

	z := &stubZuoraClient{}
	u := mock_users.NewMockUsersClient(ctrl)
	u.EXPECT().
		GetBillableOrganizations(gomock.Any(), gomock.Any()).
		Return(&users.GetBillableOrganizationsResponse{
			Organizations: organizations,
		}, nil).
		AnyTimes()

	// prepare data
	err := d.UpsertAggregates(ctx, aggregates)
	assert.NoError(t, err)

	_, err = d.InsertUsageUpload(ctx, "zuora", 1)
	assert.NoError(t, err)

	{ // zuora upload
		j := job.NewUsageUpload(d, u, usage.NewZuora(z), instrument.NewJobCollector("foo"))
		err = j.Do()
		assert.NoError(t, err)
		bcsv, err := ioutil.ReadAll(z.uploadUsage)
		assert.NoError(t, err)

		records, err := csv.NewReader(bytes.NewReader(bcsv)).ReadAll()
		assert.NoError(t, err)
		assert.Len(t, records, 3) // headers + two rows

		idxAcc := 0
		idxQty := 2
		assert.Equal(t, "Ppartial-max_aggregates_id", records[1][idxAcc])
		assert.Equal(t, "2", records[1][idxQty])
		assert.Equal(t, "Ppartial-trial_expires_at", records[2][idxAcc])
		assert.Equal(t, "4", records[2][idxQty])

		aggID, err := d.GetUsageUploadLargestAggregateID(ctx, "zuora")
		assert.NoError(t, err)
		assert.Equal(t, 4, aggID) // latest id of picked aggregation
	}
	{
		cl := &stubControlClient{}
		j := job.NewUsageUpload(d, u, usage.NewGCP(cl), instrument.NewJobCollector("foo"))
		err = j.Do()
		assert.NoError(t, err)
		assert.Len(t, cl.operations, 2)
		first := cl.operations[0]
		second := cl.operations[1]

		// Proper assignment of usage
		assert.Equal(t, int64(10801),
			*first.MetricValueSets[0].MetricValues[0].Int64Value+*second.MetricValueSets[0].MetricValues[0].Int64Value)

		assert.Equal(t, "google.weave.works/standard_nodes", first.MetricValueSets[0].MetricName)
		assert.Equal(t, "project_number:123", first.ConsumerId)
		assert.Equal(t, "opid", first.OperationId)
		assert.Equal(t, "HourlyUsageUpload", first.OperationName)

		aggID, err := d.GetUsageUploadLargestAggregateID(ctx, "gcp")
		assert.NoError(t, err)
		assert.Equal(t, 6, aggID) // latest id of picked aggregation

	}
}

func TestJobUpload_Do_zuoraError(t *testing.T) {
	d := dbtest.Setup(t)
	defer dbtest.Cleanup(t, d)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()

	z := &stubZuoraClient{err: errors.New("BOOM")}
	u := mock_users.NewMockUsersClient(ctrl)
	u.EXPECT().
		GetBillableOrganizations(gomock.Any(), gomock.Any()).
		Return(&users.GetBillableOrganizationsResponse{
			Organizations: []users.Organization{{ID: "100", ExternalID: "foo-bar-999", ZuoraAccountNumber: "Wfoo"}},
		}, nil)
	ns := "node-seconds"
	aggregates := []db.Aggregate{
		{
			BucketStart: time.Now().Truncate(24 * time.Hour).Add(-1 * 24 * time.Hour),
			InstanceID:  "100",
			AmountType:  ns,
			AmountValue: 1,
		},
	}
	err := d.UpsertAggregates(ctx, aggregates)
	assert.NoError(t, err)
	maxAggregateID, err := d.GetUsageUploadLargestAggregateID(ctx, "zuora")

	j := job.NewUsageUpload(d, u, usage.NewZuora(z), instrument.NewJobCollector("foo"))
	err = j.Do()
	assert.Error(t, err)

	aggID, err := d.GetUsageUploadLargestAggregateID(ctx, "zuora")
	assert.NoError(t, err)
	// Make sure the max_aggregate_id was removed after failing to upload
	assert.Equal(t, maxAggregateID, aggID)
}
