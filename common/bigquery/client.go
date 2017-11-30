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
)

const aggQuery = `
SELECT
  internal_instance_id AS InstanceID,
  USEC_TO_TIMESTAMP(UTC_USEC_TO_HOUR(TIMESTAMP_TO_USEC(received_at))) AS BucketStart,
  amount_type AS AmountType,
  SUM(amount_value) as AmountValue
FROM
  %s
WHERE
  received_at IS NOT NULL
  AND received_at > "%s"
GROUP BY
  InstanceID,
  AmountType,
  BucketStart
`

var queryCollector = instrument.NewHistogramCollector(prometheus.NewHistogramVec(prometheus.HistogramOpts{
	Namespace: "billing",
	Subsystem: "aggregator",
	Name:      "bigquery_query_duration_seconds",
	Help:      "Time spent running queries.",
	Buckets:   prometheus.DefBuckets,
}, instrument.HistogramCollectorBuckets))

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

// Client is a Google BigQuery client.
type Client struct {
	cfg    Config
	client *bigquery.Client
}

// New instantiates a Client.
func New(ctx context.Context, cfg Config) (*Client, error) {
	client, err := bigquery.NewClient(ctx, cfg.Project, option.WithCredentialsFile(cfg.ServiceAccountFile))
	if err != nil {
		return nil, err
	}

	return &Client{
		cfg:    cfg,
		client: client,
	}, nil
}

// Query returns a slice of the results of a query.
func (c *Client) Query(ctx context.Context, since time.Time) ([]db.Aggregate, error) {
	var result []db.Aggregate
	if err := instrument.CollectedRequest(ctx, "bigquery.Client.Query", queryCollector, nil, func(ctx context.Context) error {
		query := c.client.Query(fmt.Sprintf(aggQuery, c.cfg.DatasetAndTable, since.Format("2006-01-02 15:04:05 MST")))
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
