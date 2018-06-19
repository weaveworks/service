package job_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/weaveworks/common/instrument"
	"github.com/weaveworks/service/billing-aggregator/job"
	"github.com/weaveworks/service/billing-api/db"
	"github.com/weaveworks/service/billing-api/db/dbtest"
)

func TestAggregateDoShouldSubtractPreviouslySeenUsage(t *testing.T) {
	d := dbtest.Setup(t)
	defer dbtest.Cleanup(t, d)

	bucket := time.Date(2018, 06, 15, 9, 0, 0, 0, time.UTC)
	instanceID := "10129"
	bigQueryAggregateAtT0 := db.Aggregate{
		BucketStart: bucket,
		InstanceID:  instanceID,
		AmountType:  "node-seconds",
		AmountValue: 13620,
	}
	bigQueryAggregateAtT1 := db.Aggregate{
		BucketStart: bucket,
		InstanceID:  instanceID,
		AmountType:  "node-seconds",
		AmountValue: 17412, // 3792 additional node-seconds have been recorded since t0.
	}
	bigQueryAggregateAtT2 := db.Aggregate{
		BucketStart: bucket,
		InstanceID:  instanceID,
		AmountType:  "node-seconds",
		AmountValue: 17412, // No new usage has been recorded since t1.
	}

	jobCollector := instrument.NewJobCollector("billing_TestAggregateDo")
	client := &mockBigQueryClient{
		Queue: [][]db.Aggregate{
			{bigQueryAggregateAtT0},
			{bigQueryAggregateAtT1},
			{bigQueryAggregateAtT2},
		},
	}
	a := job.NewAggregate(client, d, jobCollector)

	// Process BigQuery aggregates at t0.
	// This should insert a first record in our DB, with the usage seen so far.
	assert.NoError(t, a.Do(&bucket))
	actualAggregates, err := d.GetAggregates(context.Background(), instanceID, bucket, bucket.Add(1*time.Hour))
	assert.NoError(t, err)
	assert.Equal(t, 1, len(actualAggregates))
	actualAggregate := actualAggregates[0]
	assert.Equal(t, bigQueryAggregateAtT0.BucketStart, actualAggregate.BucketStart)
	assert.Equal(t, bigQueryAggregateAtT0.InstanceID, actualAggregate.InstanceID)
	assert.Equal(t, bigQueryAggregateAtT0.AmountType, actualAggregate.AmountType)
	assert.Equal(t, bigQueryAggregateAtT0.AmountValue, actualAggregate.AmountValue)

	// Process BigQuery aggregates at t1.
	// This should insert a second record in our DB, with the difference between what's saved in BigQuery and what's saved in our DB.
	assert.NoError(t, a.Do(&bucket))
	actualAggregates, err = d.GetAggregates(context.Background(), instanceID, bucket, bucket.Add(1*time.Hour))
	assert.NoError(t, err)
	assert.Equal(t, 2, len(actualAggregates))
	actualAggregate = actualAggregates[0]
	assert.Equal(t, bigQueryAggregateAtT0.BucketStart, actualAggregate.BucketStart)
	assert.Equal(t, bigQueryAggregateAtT0.InstanceID, actualAggregate.InstanceID)
	assert.Equal(t, bigQueryAggregateAtT0.AmountType, actualAggregate.AmountType)
	assert.Equal(t, bigQueryAggregateAtT0.AmountValue, actualAggregate.AmountValue)
	actualAggregate = actualAggregates[1]
	assert.Equal(t, bigQueryAggregateAtT1.BucketStart, actualAggregate.BucketStart)
	assert.Equal(t, bigQueryAggregateAtT1.InstanceID, actualAggregate.InstanceID)
	assert.Equal(t, bigQueryAggregateAtT1.AmountType, actualAggregate.AmountType)
	assert.Equal(t, bigQueryAggregateAtT1.AmountValue-bigQueryAggregateAtT0.AmountValue, actualAggregate.AmountValue)

	// Process BigQuery aggregates at t2.
	// This shouldn't insert anything new, and effectively be a no-op.
	assert.NoError(t, a.Do(&bucket))
	actualAggregates, err = d.GetAggregates(context.Background(), instanceID, bucket, bucket.Add(1*time.Hour))
	assert.NoError(t, err)
	assert.Equal(t, 2, len(actualAggregates))
	actualAggregate = actualAggregates[0]
	assert.Equal(t, bigQueryAggregateAtT0.BucketStart, actualAggregate.BucketStart)
	assert.Equal(t, bigQueryAggregateAtT0.InstanceID, actualAggregate.InstanceID)
	assert.Equal(t, bigQueryAggregateAtT0.AmountType, actualAggregate.AmountType)
	assert.Equal(t, bigQueryAggregateAtT0.AmountValue, actualAggregate.AmountValue)
	actualAggregate = actualAggregates[1]
	assert.Equal(t, bigQueryAggregateAtT1.BucketStart, actualAggregate.BucketStart)
	assert.Equal(t, bigQueryAggregateAtT1.InstanceID, actualAggregate.InstanceID)
	assert.Equal(t, bigQueryAggregateAtT1.AmountType, actualAggregate.AmountType)
	assert.Equal(t, bigQueryAggregateAtT1.AmountValue-bigQueryAggregateAtT0.AmountValue, actualAggregate.AmountValue)
}

type mockBigQueryClient struct {
	// Aggregates returned every time Client#Aggregates is called:
	Queue [][]db.Aggregate
}

func (c *mockBigQueryClient) Aggregates(_ context.Context, _ time.Time) ([]db.Aggregate, error) {
	if len(c.Queue) == 0 {
		return nil, errors.New("no such element")
	}
	head := c.Queue[0]
	c.Queue = c.Queue[1:]
	return head, nil
}
