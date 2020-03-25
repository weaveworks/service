// Program to read Cortex usage (samples sent per user) and upload to the billing system
package main

import (
	"context"
	"flag"
	"fmt"
	"math"
	"net/http"
	"time"

	promApi "github.com/prometheus/client_golang/api"
	promV1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/robfig/cron"
	"github.com/sirupsen/logrus"

	billing "github.com/weaveworks/billing-client"
	"github.com/weaveworks/common/instrument"
	"github.com/weaveworks/common/logging"
	"github.com/weaveworks/common/mtime"
	"github.com/weaveworks/common/server"
)

var jobCollector = instrument.NewJobCollector("usage")

func init() {
	jobCollector.Register()
	billing.MustRegisterMetrics()
}

func main() {
	var (
		serverConfig  server.Config
		billingConfig billing.Config
		url           string
	)
	flag.StringVar(&url, "metrics.url", "", "Prometheus query URL")
	serverConfig.RegisterFlags(flag.CommandLine)
	billingConfig.RegisterFlags(flag.CommandLine)
	flag.Parse()
	serverConfig.MetricsNamespace = "usage"

	server, err := server.New(serverConfig)
	checkFatal(err)
	defer server.Shutdown()

	billingClient, err := billing.NewClient(billingConfig)
	checkFatal(err)
	promClient, err := promApi.NewClient(promApi.Config{Address: url})
	checkFatal(err)

	metricsJob, err := newMetricsJob(server.Log, billingClient, promClient)
	checkFatal(err)

	metricsCron := cron.New()
	metricsCron.AddJob("0 * * * * *", metricsJob) // every minute
	metricsCron.Start()
	defer metricsCron.Stop()

	// healthCheck handles a very simple health check
	server.HTTP.Path("/healthcheck").Methods("GET").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	server.Run()
}

type metricsJob struct {
	log           logging.Interface
	billingClient *billing.Client
	promAPI       promV1.API
}

func newMetricsJob(log logging.Interface, billingClient *billing.Client, promClient promApi.Client) (cron.Job, error) {
	return &metricsJob{
		log:           log,
		billingClient: billingClient,
		promAPI:       promV1.NewAPI(promClient),
	}, nil
}

// This function will get run periodically by the cron package
func (m *metricsJob) Run() {
	ctx := context.Background()
	now := mtime.Now()
	// Note 'user' here means instance in Weave-Cloud-speak.
	m.queryAndEmit(ctx, now, "samples", "", `sum by (user)(increase(cortex_distributor_received_samples_total{job="cortex/distributor"}[1m]))`)
	m.queryAndEmit(ctx, now, "storage-bytes", "-cortex", `sum by (user)(increase(cortex_ingester_chunk_stored_bytes_total{job="cortex/ingester"}[1m]))`)
	m.queryAndEmit(ctx, now, "storage-bytes", "-scope", `sum by (user)(increase(scope_reports_bytes_total{job="scope/collection"}[1m]))`)
}

// Run a Prom query and emit one billing record for each point that comes back
func (m *metricsJob) queryAndEmit(ctx context.Context, now time.Time, amountType billing.AmountType, tag, query string) {
	vector, err := m.promQuery(ctx, now, query)
	if err != nil {
		m.log.Warnf("error from query %q: %s", query, err)
		return
	}
	for _, sample := range vector {
		if sample.Value == 0 || math.IsNaN(float64(sample.Value)) {
			continue
		}
		user := string(sample.Metric["user"])
		if user == "" {
			m.log.Warnf("User not found in query result: %v", vector)
			return
		}
		uniqueKey := user + ":" + fmt.Sprint(sample.Timestamp) + tag
		m.emitBillingRecord(ctx, now, amountType, user, uniqueKey, int64(sample.Value))
	}
}

// Run a Prom query and check that we get the expected type
func (m *metricsJob) promQuery(ctx context.Context, now time.Time, query string) (model.Vector, error) {
	value, err := m.promAPI.Query(ctx, query, now)
	if err != nil {
		return nil, err
	}
	vector, ok := value.(model.Vector)
	if !ok {
		return nil, fmt.Errorf("Unable to convert value to vector: %#v", value)
	}
	return vector, nil
}

func (m *metricsJob) emitBillingRecord(ctx context.Context, now time.Time, amountType billing.AmountType, userID, uniqueKey string, samples int64) {
	amounts := billing.Amounts{
		amountType: samples,
	}
	err := m.billingClient.AddAmounts(uniqueKey, userID, now, amounts, nil)
	if err != nil {
		m.log.Warnf("error sending billing data: %s", err)
	}
}

func checkFatal(e error) {
	if e != nil {
		logrus.Fatal(e)
	}
}
