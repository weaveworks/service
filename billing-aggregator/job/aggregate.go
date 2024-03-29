package job

import (
	log "github.com/sirupsen/logrus"

	"context"
	"time"

	"github.com/weaveworks/common/instrument"
	"github.com/weaveworks/service/billing-api/db"
	"github.com/weaveworks/service/common/bigquery"
)

const batchSize = 100

// Aggregate reads events from bigquery and stores them in the database.
type Aggregate struct {
	bigqueryClient bigquery.Client
	db             db.DB
	collector      *instrument.JobCollector
}

// NewAggregate creates an Aggregate instance.
func NewAggregate(bigquery bigquery.Client, db db.DB, collector *instrument.JobCollector) *Aggregate {
	return &Aggregate{
		bigqueryClient: bigquery,
		db:             db,
		collector:      collector,
	}
}

// Run starts the job and logs errors.
func (j *Aggregate) Run() {
	if err := j.Do(nil); err != nil {
		log.Errorf("Error running job: %v", err)
	}
}

// Do starts the job and returns an error if it fails.
func (j *Aggregate) Do(since *time.Time) error {
	var t time.Time
	if since == nil {
		// Default is to check for updated totals for the last 6 hours (rounded to the previous full hour)
		t = time.Now().UTC().Add(-6 * time.Hour).Truncate(time.Hour)
	} else {
		// Ensure any custom time is aligned to the hour, so we don't generate partial totals
		t = since.UTC().Truncate(time.Hour)
	}
	since = &t
	return instrument.CollectedRequest(context.Background(), "Aggregate.Do", j.collector, nil, func(ctx context.Context) error {
		aggs, err := j.bigqueryClient.Aggregates(ctx, *since)
		if err != nil {
			return err
		}

		log.Infof("Received %d records from BigQuery since %q", len(aggs), since)

		for _, agg := range aggs {
			log.Debugf("%+v", agg)
		}

		for i := 0; i < len(aggs); i += batchSize {
			l := i + batchSize
			if l > len(aggs) {
				l = len(aggs)
			}

			bqAggs := aggs[i:l]
			instanceIDs := instanceIDs(bqAggs)
			dbAggs, err := j.db.GetAggregatesFrom(ctx, instanceIDs, *since)
			if err != nil {
				return err
			}
			batch := subtract(bqAggs, dbAggs)

			if err := j.db.InsertAggregates(ctx, batch); err != nil {
				return err
			}
		}

		log.Infof("Inserted %d records into database", len(aggs))
		return nil
	})
}

func instanceIDs(aggs []db.Aggregate) []string {
	ids := make([]string, len(aggs))
	for _, agg := range aggs {
		ids = append(ids, agg.InstanceID)
	}
	return ids
}

func subtract(bqAggs, dbAggs []db.Aggregate) []db.Aggregate {
	aggs := []db.Aggregate{}
	sums := sumByKey(dbAggs)
	for _, bqAgg := range bqAggs {
		sum := sums[asKey(bqAgg)]
		bqAgg.AmountValue -= sum
		if bqAgg.AmountValue > 0 {
			aggs = append(aggs, bqAgg)
		}
	}
	return aggs
}

type key struct {
	InstanceID  string
	BucketStart time.Time
	AmountType  string
}

func asKey(agg db.Aggregate) key {
	return key{
		BucketStart: agg.BucketStart,
		InstanceID:  agg.InstanceID,
		AmountType:  agg.AmountType,
	}
}

func sumByKey(aggs []db.Aggregate) map[key]int64 {
	m := make(map[key]int64)
	for _, agg := range aggs {
		m[asKey(agg)] += agg.AmountValue
	}
	return m
}
