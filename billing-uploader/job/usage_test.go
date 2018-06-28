package job_test

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"io"
	"io/ioutil"
	"strconv"
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

func (z *stubZuoraClient) GetAccount(ctx context.Context, zuoraAccountNumber string) (*zuora.Account, error) {
	return &zuora.Account{
		PaymentProviderID: "P" + zuoraAccountNumber,
		Subscription: &zuora.AccountSubscription{
			SubscriptionNumber: "S" + zuoraAccountNumber,
			ChargeNumber:       "C" + zuoraAccountNumber,
		},
	}, nil
}

func (z *stubZuoraClient) GetProductsUnitSet(ctx context.Context, productIDs []string) (map[string]bool, error) {
	return map[string]bool{
		"node-seconds": true,
	}, nil
}

func (z *stubZuoraClient) UploadUsage(ctx context.Context, r io.Reader, id string) (string, error) {
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
	return name
}

var (
	start         = time.Date(2017, 11, 27, 0, 0, 0, 0, time.UTC)
	now           = start.Add(2 * 24 * time.Hour)
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
	lastAggregateProcessed = db.Aggregate{
		// ID==max_aggregate_id; skip: <=max_aggregate_id
		BucketStart: start, // 2017-11-27 00:00
		InstanceID:  "100",
		AmountType:  "node-seconds",
		AmountValue: 1,
	}
	aggregates = []db.Aggregate{
		// Zuora
		// skip: <=max_aggregate_id
		lastAggregateProcessed,
		{ // pick
			BucketStart: start.Add(1 * time.Hour), // 2017-11-27 01:00
			InstanceID:  "100",
			AmountType:  "node-seconds",
			AmountValue: 2,
		},
		{ // skip: <trial_expires_at
			BucketStart: expires.Add(-1 * time.Minute),
			InstanceID:  "101",
			AmountType:  "node-seconds",
			AmountValue: 3,
		},
		{ // pick: >trial_expires_at
			BucketStart: expires.Add(1 * time.Minute),
			InstanceID:  "101",
			AmountType:  "node-seconds",
			AmountValue: 4,
		},

		// GCP
		{ // pick
			BucketStart: start,
			InstanceID:  "200",
			AmountType:  "node-seconds",
			AmountValue: 10800,
		},
		{ // pick
			BucketStart: start.Add(1 * time.Hour),
			InstanceID:  "200",
			AmountType:  "node-seconds",
			AmountValue: 1,
		},
		{ // skip
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
	err := d.InsertAggregates(ctx, aggregates)
	assert.NoError(t, err)

	maxAggregateID := maxAggregateID(t, d)
	_, err = d.InsertUsageUpload(ctx, "zuora", maxAggregateID)
	assert.NoError(t, err)

	{ // zuora upload
		j := job.NewUsageUpload(d, u, usage.NewZuora(z), instrument.NewJobCollector("foo"))
		err = j.Do(now)
		assert.NoError(t, err)
		bcsv, err := ioutil.ReadAll(z.uploadUsage)
		assert.NoError(t, err)

		records, err := csv.NewReader(bytes.NewReader(bcsv)).ReadAll()
		assert.NoError(t, err)
		assert.Len(t, records, 3) // headers + two rows

		assert.Equal(t, []string{
			"PWboo",
			"node-seconds",
			"2",
			"11/27/2017", // day of first aggregate
			"11/29/2017",
			"SWboo",
			"CWboo",
		}, records[1][0:7])
		assert.Equal(t, []string{
			"PWzoo",
			"node-seconds",
			"4",
			"11/25/2017",
			"11/29/2017",
			"SWzoo",
			"CWzoo",
		}, records[2][0:7])

		aggID, err := d.GetUsageUploadLargestAggregateID(ctx, "zuora")
		assert.NoError(t, err)
		assert.Equal(t, maxAggregateID+3, aggID) // latest id of picked aggregation
	}
	{ // gcp upload
		cl := &stubControlClient{}
		j := job.NewUsageUpload(d, u, usage.NewGCP(cl), instrument.NewJobCollector("foo"))
		err = j.Do(now)
		assert.NoError(t, err)
		assert.Len(t, cl.operations, 2)

		ops := map[string]*servicecontrol.Operation{} // map[ID]op
		for _, op := range cl.operations {
			ops[op.OperationId] = op
		}

		firstGCPAggID := maxAggregateID + 4
		firstGCPAggIDStr := strconv.Itoa(firstGCPAggID)
		secondGCPAggID := maxAggregateID + 5
		secondGCPAggIDStr := strconv.Itoa(secondGCPAggID)
		five := ops[firstGCPAggIDStr]
		six := ops[secondGCPAggIDStr]
		assert.NotNil(t, five, "ID="+firstGCPAggIDStr+" was not picked")
		assert.NotNil(t, six, "ID="+secondGCPAggIDStr+" was not picked")

		// Proper assignment of usage
		assert.Equal(t, int64(10800), *five.MetricValueSets[0].MetricValues[0].Int64Value)
		assert.Equal(t, int64(1), *six.MetricValueSets[0].MetricValues[0].Int64Value)

		// org related
		assert.Equal(t, "google.weave.works/standard_nodes", five.MetricValueSets[0].MetricName)
		assert.Equal(t, "project_number:123", five.ConsumerId)
		assert.Equal(t, "HourlyUsageUpload", five.OperationName)

		// bucket related
		assert.Equal(t, firstGCPAggIDStr, five.OperationId)
		assert.Equal(t, start.Format(time.RFC3339), five.StartTime)
		assert.Equal(t, start.Add(1*time.Hour).Format(time.RFC3339), five.EndTime)

		assert.Equal(t, secondGCPAggIDStr, six.OperationId)
		assert.Equal(t, start.Add(1*time.Hour).Format(time.RFC3339), six.StartTime)
		assert.Equal(t, start.Add(2*time.Hour).Format(time.RFC3339), six.EndTime)

		aggID, err := d.GetUsageUploadLargestAggregateID(ctx, "gcp")
		assert.NoError(t, err)
		assert.Equal(t, secondGCPAggID, aggID) // latest id of picked aggregation

		// Add one more aggregate and make sure this second run has successfully
		// reset the previous report
		err = d.InsertAggregates(ctx, []db.Aggregate{
			{ // ID==7
				BucketStart: start.Add(2 * time.Hour),
				InstanceID:  "200",
				AmountType:  "node-seconds",
				AmountValue: 999,
			},
		})
		assert.NoError(t, err)
		err = j.Do(now)
		assert.NoError(t, err)
		assert.Len(t, cl.operations, 1)
	}
}

func maxAggregateID(t *testing.T, d db.DB) int {
	instanceID := lastAggregateProcessed.InstanceID
	from := lastAggregateProcessed.BucketStart
	to := lastAggregateProcessed.BucketStart.Add(1 * time.Hour)
	aggs, err := d.GetAggregates(context.Background(), instanceID, from, to)
	assert.NoError(t, err)
	maxAggregateID := -1
	for _, agg := range aggs {
		if isLastAggregateProcessed(agg) {
			maxAggregateID = agg.ID
		}
	}
	assert.True(t, maxAggregateID > 0)
	return maxAggregateID
}

func isLastAggregateProcessed(agg db.Aggregate) bool {
	return agg.BucketStart == lastAggregateProcessed.BucketStart &&
		agg.InstanceID == lastAggregateProcessed.InstanceID &&
		agg.AmountType == lastAggregateProcessed.AmountType &&
		agg.AmountValue == lastAggregateProcessed.AmountValue
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
	aggregates := []db.Aggregate{
		{
			BucketStart: now.Add(-1 * 24 * time.Hour),
			InstanceID:  "100",
			AmountType:  "node-seconds",
			AmountValue: 1,
		},
	}
	err := d.InsertAggregates(ctx, aggregates)
	assert.NoError(t, err)
	maxAggregateID, err := d.GetUsageUploadLargestAggregateID(ctx, "zuora")

	j := job.NewUsageUpload(d, u, usage.NewZuora(z), instrument.NewJobCollector("foo"))
	err = j.Do(now)
	assert.Error(t, err)

	aggID, err := d.GetUsageUploadLargestAggregateID(ctx, "zuora")
	assert.NoError(t, err)
	// Make sure the max_aggregate_id was removed after failing to upload
	assert.Equal(t, maxAggregateID, aggID)
}
