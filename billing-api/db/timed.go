package db

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/weaveworks/common/instrument"
	"github.com/weaveworks/service/common/billing/grpc"
)

var durationCollector = instrument.NewHistogramCollector(prometheus.NewHistogramVec(prometheus.HistogramOpts{
	Namespace: "billing",
	Name:      "db_duration_seconds",
	Help:      "Time spent talking to the DB.",
	Buckets:   prometheus.DefBuckets,
}, instrument.HistogramCollectorBuckets))

func init() {
	durationCollector.Register()
}

// timed adds prometheus timings to another database implementation
type timed struct {
	d DB
}

func (t timed) timeRequest(ctx context.Context, method string, f func(context.Context) error) error {
	return instrument.CollectedRequest(ctx, method, durationCollector, nil, f)
}

func (t timed) InsertAggregates(ctx context.Context, aggregates []Aggregate) error {
	return t.timeRequest(ctx, "InsertAggregates", func(ctx context.Context) error {
		return t.d.InsertAggregates(ctx, aggregates)
	})
}

func (t timed) GetAggregates(ctx context.Context, instanceID string, from, through time.Time) (as []Aggregate, err error) {
	t.timeRequest(ctx, "GetAggregates", func(ctx context.Context) error {
		as, err = t.d.GetAggregates(ctx, instanceID, from, through)
		return err
	})
	return
}

func (t timed) GetAggregatesToUpload(ctx context.Context, instanceID string, from, through time.Time) (as []Aggregate, err error) {
	t.timeRequest(ctx, "GetAggregatesToUpload", func(ctx context.Context) error {
		as, err = t.d.GetAggregatesToUpload(ctx, instanceID, from, through)
		return err
	})
	return
}

func (t timed) GetAggregatesUploaded(ctx context.Context, uploadID int64) (as []Aggregate, err error) {
	t.timeRequest(ctx, "GetAggregatesUploaded", func(ctx context.Context) error {
		as, err = t.d.GetAggregatesUploaded(ctx, uploadID)
		return err
	})
	return
}

func (t timed) GetAggregatesFrom(ctx context.Context, instanceIDs []string, from time.Time) (as []Aggregate, err error) {
	t.timeRequest(ctx, "GetAggregatesFrom", func(ctx context.Context) error {
		as, err = t.d.GetAggregatesFrom(ctx, instanceIDs, from)
		return err
	})
	return
}

func (t timed) GetLatestUsageUpload(ctx context.Context, uploader string) (upload *UsageUpload, err error) {
	t.timeRequest(ctx, "GetLatestUsageUpload", func(ctx context.Context) error {
		upload, err = t.d.GetLatestUsageUpload(ctx, uploader)
		return err
	})
	return
}

func (t timed) InsertUsageUpload(ctx context.Context, uploader string, aggregateIDs []int) (uploadID int64, err error) {
	t.timeRequest(ctx, "InsertUsageUpload", func(ctx context.Context) error {
		uploadID, err = t.d.InsertUsageUpload(ctx, uploader, aggregateIDs)
		return err
	})
	return
}

func (t timed) DeleteUsageUpload(ctx context.Context, uploader string, aggregateID int64) (err error) {
	t.timeRequest(ctx, "DeleteUsageUpload", func(ctx context.Context) error {
		err = t.d.DeleteUsageUpload(ctx, uploader, aggregateID)
		return err
	})
	return
}

func (t timed) GetMonthSums(ctx context.Context, instanceIDs []string, from, through time.Time) (as map[string][]Aggregate, err error) {
	t.timeRequest(ctx, "GetMonthSums", func(ctx context.Context) error {
		as, err = t.d.GetMonthSums(ctx, instanceIDs, from, through)
		return err
	})
	return
}

func (t timed) InsertPostTrialInvoice(ctx context.Context, externalID, zuoraAccountNumber, usageImportID string) error {
	return t.timeRequest(ctx, "InsertPostTrialInvoice", func(ctx context.Context) error {
		return t.d.InsertPostTrialInvoice(ctx, externalID, zuoraAccountNumber, usageImportID)
	})
}

func (t timed) GetPostTrialInvoices(ctx context.Context) (pti []PostTrialInvoice, err error) {
	t.timeRequest(ctx, "GetPostTrialInvoices", func(ctx context.Context) error {
		pti, err = t.d.GetPostTrialInvoices(ctx)
		return err
	})
	return
}

func (t timed) DeletePostTrialInvoice(ctx context.Context, usageImportID string) (err error) {
	return t.timeRequest(ctx, "DeletePostTrialInvoice", func(ctx context.Context) error {
		return t.d.DeletePostTrialInvoice(ctx, usageImportID)
	})
}

func (t timed) FindBillingAccountByTeamID(ctx context.Context, teamID string) (account *grpc.BillingAccount, err error) {
	t.timeRequest(ctx, "FindBillingAccountByTeamID", func(ctx context.Context) error {
		account, err = t.d.FindBillingAccountByTeamID(ctx, teamID)
		return err
	})
	return
}

func (t timed) Transaction(f func(DB) error) error {
	// We don't time transactions as they are only used in tests
	return t.d.Transaction(f)
}

func (t timed) Close(ctx context.Context) error {
	return t.timeRequest(ctx, "Close", func(ctx context.Context) error {
		return t.d.Close(ctx)
	})
}
