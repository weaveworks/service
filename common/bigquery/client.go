package bigquery

import (
	"context"
	"flag"
	"fmt"
	"time"

	"cloud.google.com/go/bigquery"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"

	"github.com/weaveworks/common/instrument"
	"github.com/weaveworks/service/billing-api/db"
	"github.com/weaveworks/service/common"
)

const (
	aggQuery = `
SELECT
  internal_instance_id AS InstanceID,
  TIMESTAMP_TRUNC(received_at, HOUR, 'UTC') AS BucketStart,
  amount_type AS AmountType,
  SUM(amount_value) as AmountValue
FROM
  %s
WHERE
  received_at IS NOT NULL
  AND received_at > @StartTime
  AND _PARTITIONTIME >= @DateLowerLimit
GROUP BY
  InstanceID,
  AmountType,
  BucketStart
`
	sqlDateFormat = "2006-01-02 15:04:05 MST"
)

var queryCollector = instrument.NewHistogramCollectorFromOpts(prometheus.HistogramOpts{
	Namespace: common.PrometheusNamespace,
	Subsystem: "bigquery",
	Name:      "query_duration_seconds",
	Help:      "Time spent running queries.",
})

func init() {
	queryCollector.Register()
}

// Config holds settings for the bigquery client.
type Config struct {
	Project            string
	DatasetAndTable    string
	ServiceAccountFile string
}

// RegisterFlags registers configuration variables.
func (cfg *Config) RegisterFlags(f *flag.FlagSet) {
	f.StringVar(&cfg.Project, "bigquery.project", "weaveworks-bi", "BigQuery project to query.")
	f.StringVar(&cfg.DatasetAndTable, "bigquery.dataset-and-table", "billing_dev.events", "BigQuery table to query.")
	f.StringVar(&cfg.ServiceAccountFile, "bigquery.service-account-file", "", "BigQuery service account credentials file.")
}

// Client is the interface for our Google BigQuery client.
type Client interface {
	Aggregates(ctx context.Context, since time.Time) ([]db.Aggregate, error)
}

// DefaultClient is an implementation for Client, our Google BigQuery client interface.
type DefaultClient struct {
	cfg    Config
	client *bigquery.Client
}

// New instantiates a DefaultClient.
func New(ctx context.Context, cfg Config) (*DefaultClient, error) {
	client, err := bigquery.NewClient(ctx, cfg.Project, option.WithCredentialsFile(cfg.ServiceAccountFile))
	if err != nil {
		return nil, err
	}

	return &DefaultClient{
		cfg:    cfg,
		client: client,
	}, nil
}

// Aggregates returns a slice of the results of a query.
func (c *DefaultClient) Aggregates(ctx context.Context, since time.Time) ([]db.Aggregate, error) {
	var result []db.Aggregate
	if err := instrument.CollectedRequest(ctx, "bigquery.Client.Aggregates", queryCollector, nil, func(ctx context.Context) error {
		query := c.client.Query(fmt.Sprintf(aggQuery, c.cfg.DatasetAndTable))
		query.Parameters = []bigquery.QueryParameter{
			{Name: "StartTime", Value: since.Format(sqlDateFormat)},
			{Name: "DateLowerLimit", Value: since.Truncate(24 * time.Hour).Format(sqlDateFormat)},
		}
		// If you see the error "query job missing destination table", there's often actually an issue with the query
		// Enable this temporarily to see the actual error
		// query.Dst = &bigquery.Table{
		// 	ProjectID: "weaveworks-bi",
		// 	DatasetID: "service_dev",
		// 	TableID:   "test_12345",
		// }
		it, err := query.Read(ctx)
		if err != nil {
			return err
		}

		for {
			var agg db.Aggregate
			err := it.Next(&agg)
			if err == iterator.Done {
				break
			}
			if err != nil {
				return err
			}
			result = append(result, agg)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return result, nil
}
