package job

import (
	log "github.com/sirupsen/logrus"

	"context"
	"github.com/weaveworks/common/instrument"
	"github.com/weaveworks/service/billing/bigquery"
	"github.com/weaveworks/service/billing/db"
	"time"
)

const upsertBatchSize = 100

// Aggregate reads events from bigquery and stores them in the database.
type Aggregate struct {
	bigqueryClient *bigquery.Client
	db             db.DB
	collector      *instrument.JobCollector
}

// NewAggregate creates an Aggregate instance.
func NewAggregate(bigquery *bigquery.Client, db db.DB, collector *instrument.JobCollector) *Aggregate {
	return &Aggregate{
		bigqueryClient: bigquery,
		db:             db,
		collector:      collector,
	}
}

// Run starts the job and logs errors.
func (j *Aggregate) Run() {
	if err := j.Do(nil); err != nil {
		log.Errorf("Error running job %v", err)
	}
}

// Do starts the job and returns an error if it fails.
func (j *Aggregate) Do(since *time.Time) error {
	if since == nil {
		// Default is to check for updated totals for the last 6 hours (rounded to the previous full hour)
		t := time.Now().UTC().Add(-6 * time.Hour).Truncate(time.Hour)
		since = &t
	}
	return instrument.CollectedRequest(context.Background(), "Aggregate.Do", j.collector, nil, func(ctx context.Context) error {
		aggs, err := j.bigqueryClient.Query(ctx, *since)
		if err != nil {
			return err
		}

		log.Infof("Received %d records from BigQuery since %q", len(aggs), since)

		for _, agg := range aggs {
			log.Debugf("%+v", agg)
		}

		for i := 0; i < len(aggs); i += upsertBatchSize {
			l := i + upsertBatchSize
			if l > len(aggs) {
				l = len(aggs)
			}
			if err := j.db.UpsertAggregates(ctx, aggs[i:l]); err != nil {
				return err
			}
		}

		log.Infof("Inserted %d records into database", len(aggs))
		return nil
	})
}
