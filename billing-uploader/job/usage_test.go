package job_test

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"github.com/stretchr/testify/require"
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
			ZuoraAccountNumber: "Wskip-empty_id",
			ExternalID:         "skip-empty_id",
		},
		{
			ID:                 "100",
			ZuoraAccountNumber: "Wpartial-upload",
			ExternalID:         "partial-upload",
		},
		{
			ID:                 "101",
			ZuoraAccountNumber: "Wpartial-trial_expires_at",
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
	uploadedAggregate = db.Aggregate{
		// skip: UploadID!=0
		BucketStart: start, // 2017-11-27 00:00
		InstanceID:  "100",
		AmountType:  "node-seconds",
		AmountValue: 1,
		UploadID:    -1,
	}
	aggregates = []db.Aggregate{
		// Zuora
		// skip: UploadID!=0
		uploadedAggregate,
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
			AmountValue: 11,
		},
		{ // skip
			BucketStart: start,
			InstanceID:  "201",
			AmountType:  "node-seconds",
			AmountValue: 12,
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
			"PWpartial-upload",
			"node-seconds",
			"2",
			"11/27/2017", // day of first aggregate
			"11/29/2017",
			"SWpartial-upload",
			"CWpartial-upload",
		}, records[1][0:7])
		assert.Equal(t, []string{
			"PWpartial-trial_expires_at",
			"node-seconds",
			"4",
			"11/25/2017",
			"11/29/2017",
			"SWpartial-trial_expires_at",
			"CWpartial-trial_expires_at",
		}, records[2][0:7])

		upload, err := d.GetLatestUsageUpload(ctx, "")
		assert.NoError(t, err)
		aggs, err := d.GetAggregatesUploaded(ctx, upload.ID)
		assert.NoError(t, err)
		assert.Len(t, aggs, 2)
		for _, a := range aggs {
			switch a.InstanceID {
			case "100":
				assert.Equal(t, int64(2), a.AmountValue)
			case "101":
				assert.Equal(t, int64(4), a.AmountValue)
			default:
				assert.Fail(t, "unexpected instance id: %s", a.InstanceID)
			}
		}
	}
	{ // gcp upload
		cl := &stubControlClient{}
		j := job.NewUsageUpload(d, u, usage.NewGCP(cl), instrument.NewJobCollector("foo"))
		err = j.Do(now)
		assert.NoError(t, err)
		assert.Len(t, cl.operations, 2)
		upload, err := d.GetLatestUsageUpload(ctx, "")
		assert.NoError(t, err)
		uploadedAggs, err := d.GetAggregatesUploaded(ctx, upload.ID)
		assert.Len(t, uploadedAggs, 2)

		ops := map[string]*servicecontrol.Operation{} // map[ID]op
		for _, op := range cl.operations {
			ops[op.OperationId] = op
		}

		firstGCPAggIDStr := strconv.Itoa(uploadedAggs[0].ID)
		secondGCPAggIDStr := strconv.Itoa(uploadedAggs[1].ID)
		five := ops[firstGCPAggIDStr]
		six := ops[secondGCPAggIDStr]
		assert.NotNil(t, five, "ID="+firstGCPAggIDStr+" was not picked")
		assert.NotNil(t, six, "ID="+secondGCPAggIDStr+" was not picked")

		// Proper assignment of usage
		assert.Equal(t, int64(10800), *five.MetricValueSets[0].MetricValues[0].Int64Value)
		assert.Equal(t, int64(11), *six.MetricValueSets[0].MetricValues[0].Int64Value)

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

		upload, err = d.GetLatestUsageUpload(ctx, "")
		assert.NoError(t, err)
		aggs, err := d.GetAggregatesUploaded(ctx, upload.ID)
		assert.NoError(t, err)
		assert.Len(t, aggs, 1)
		assert.Equal(t, int64(999), aggs[0].AmountValue)
	}
}

func TestJobUpload_Do_zuoraError(t *testing.T) {
	d := dbtest.Setup(t)
	defer dbtest.Cleanup(t, d)

	ctx := context.Background()
	uploadBefore := getLatestUpload(t, d)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	z := &stubZuoraClient{err: errors.New("BOOM")}
	u := mock_users.NewMockUsersClient(ctrl)
	u.EXPECT().
		GetBillableOrganizations(gomock.Any(), gomock.Any()).
		Return(&users.GetBillableOrganizationsResponse{
			Organizations: []users.Organization{{ID: "100", ExternalID: "foo-bar-999", ZuoraAccountNumber: "Wskip-empty_id"}},
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

	j := job.NewUsageUpload(d, u, usage.NewZuora(z), instrument.NewJobCollector("foo"))
	err = j.Do(now)
	assert.Error(t, err)

	// Check there are no new uploads
	upload := getLatestUpload(t, d)
	assert.Equal(t, uploadBefore.ID, upload.ID)

	// Check that any assigned usage was cleared
	aggs, err := d.GetAggregatesUploaded(ctx, upload.ID+1)
	assert.NoError(t, err)
	assert.Len(t, aggs, 0)
}

func TestJobUpload_Do_outOfOrder(t *testing.T) {
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
			Organizations: []users.Organization{
				{
					ID:                 "100",
					ZuoraAccountNumber: "Wtest100",
					ExternalID:         "test100",
				},
				{
					ID:                 "101",
					ZuoraAccountNumber: "Wtest101",
					ExternalID:         "test101",
				},
			},
		}, nil).
		AnyTimes()

	firstDayStart := start
	secondDayStart := firstDayStart.Add(24 * time.Hour)
	thirdDayStart := secondDayStart.Add(24 * time.Hour)

	aggregates = []db.Aggregate{
		{
			BucketStart: firstDayStart.Add(1 * time.Hour), // 2017-11-27 01:00
			InstanceID:  "100",
			AmountType:  "node-seconds",
			AmountValue: 1,
		},
		{
			BucketStart: secondDayStart.Add(1 * time.Hour),
			InstanceID:  "101",
			AmountType:  "node-seconds",
			AmountValue: 2,
		},
		{
			BucketStart: secondDayStart.Add(-1 * time.Hour),
			InstanceID:  "100",
			AmountType:  "node-seconds",
			AmountValue: 3,
		},
	}
	err := d.InsertAggregates(ctx, aggregates)
	assert.NoError(t, err)

	j := job.NewUsageUpload(d, u, usage.NewZuora(z), instrument.NewJobCollector("foo"))
	err = j.Do(secondDayStart.Add(10 * time.Minute))
	assert.NoError(t, err)

	// do usage upload, check next_day usage is not included
	upload, err := d.GetLatestUsageUpload(ctx, "zuora")
	assert.NoError(t, err)
	aggs, err := d.GetAggregatesUploaded(ctx, upload.ID)
	assert.NoError(t, err)
	assert.Len(t, aggs, 2)

	err = d.InsertAggregates(ctx, []db.Aggregate{
		{
			BucketStart: secondDayStart.Add(2 * time.Hour),
			InstanceID:  "101",
			AmountType:  "node-seconds",
			AmountValue: 4,
		},
	})
	assert.NoError(t, err)

	// do usage upload for next_day, check usage is included
	err = j.Do(thirdDayStart.Add(10 * time.Minute))
	upload, err = d.GetLatestUsageUpload(ctx, "zuora")
	assert.NoError(t, err)
	aggs, err = d.GetAggregatesUploaded(ctx, upload.ID)
	assert.NoError(t, err)
	assert.Len(t, aggs, 2)
	assert.Equal(t, int64(2), aggs[0].AmountValue)
	assert.Equal(t, int64(4), aggs[1].AmountValue)

	err = d.DeleteUsageUpload(ctx, upload.Uploader, upload.ID)
	assert.NoError(t, err)
	aggs, err = d.GetAggregatesUploaded(ctx, upload.ID)
	assert.Len(t, aggs, 0)
}

// getLatestUpload provides a default placeholder usage upload entry for when there might not
// already be an entry
func getLatestUpload(t *testing.T, d db.DB) db.UsageUpload {
	usage, err := d.GetLatestUsageUpload(context.Background(), "")
	require.NoError(t, err)

	if usage == nil {
		usage = &db.UsageUpload{ID: 0, Uploader: "placeholder"}
	}
	return *usage
}
