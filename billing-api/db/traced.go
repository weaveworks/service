package db

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/weaveworks/service/common/billing/grpc"
)

// traced adds logrus trace lines on each db call
type traced struct {
	d DB
}

func (t traced) trace(name string, args ...interface{}) {
	logrus.Debugf("%s: %#v", name, args)
}

func (t traced) InsertAggregates(ctx context.Context, aggregates []Aggregate) (err error) {
	defer func() { t.trace("InsertAggregates", aggregates, err) }()
	return t.d.InsertAggregates(ctx, aggregates)
}

func (t traced) GetAggregates(ctx context.Context, instanceID string, from, through time.Time) (as []Aggregate, err error) {
	// We don't trace GetAggregates in the normal way, because it floods the debug logs with too much data.
	defer func() { t.trace("GetAggregates", instanceID, from, through, len(as), err) }()
	return t.d.GetAggregates(ctx, instanceID, from, through)
}

func (t traced) GetAggregatesToUpload(ctx context.Context, instanceID string, from, through time.Time) (as []Aggregate, err error) {
	// We don't trace GetAggregatesToUpload in the normal way, because it floods the debug logs with too much data.
	defer func() { t.trace("GetAggregatesToUpload", instanceID, from, through, len(as), err) }()
	return t.d.GetAggregatesToUpload(ctx, instanceID, from, through)
}

func (t traced) GetAggregatesUploaded(ctx context.Context, uploadID int64) (as []Aggregate, err error) {
	// We don't trace GetAggregatesUploaded in the normal way, because it floods the debug logs with too much data.
	defer func() { t.trace("GetAggregatesUploaded", uploadID, len(as), err) }()
	return t.d.GetAggregatesUploaded(ctx, uploadID)
}

func (t traced) GetAggregatesFrom(ctx context.Context, instanceIDs []string, from time.Time) (as []Aggregate, err error) {
	defer func() { t.trace("GetAggregatesFrom", instanceIDs, from, len(as), err) }()
	return t.d.GetAggregatesFrom(ctx, instanceIDs, from)
}

func (t traced) GetLatestUsageUpload(ctx context.Context, uploader string) (upload *UsageUpload, err error) {
	defer func() { t.trace("GetLatestUsageUpload", uploader, upload, err) }()
	return t.d.GetLatestUsageUpload(ctx, uploader)
}

func (t traced) InsertUsageUpload(ctx context.Context, uploader string, aggregateIDs []int) (uploadID int64, err error) {
	defer func() { t.trace("InsertUsageUpload", uploader, aggregateIDs, uploadID, err) }()
	return t.d.InsertUsageUpload(ctx, uploader, aggregateIDs)
}

func (t traced) DeleteUsageUpload(ctx context.Context, uploader string, uploadID int64) (err error) {
	defer func() { t.trace("DeleteUsageUpload", uploader, uploadID, err) }()
	return t.d.DeleteUsageUpload(ctx, uploader, uploadID)
}

func (t traced) GetMonthSums(ctx context.Context, instanceIDs []string, from, through time.Time) (as map[string][]Aggregate, err error) {
	defer func() { t.trace("GetMonthSums", instanceIDs, from, through, as, err) }()
	return t.d.GetMonthSums(ctx, instanceIDs, from, through)
}

func (t traced) InsertPostTrialInvoice(ctx context.Context, externalID, zuoraAccountNumber, usageImportID string) (err error) {
	defer func() { t.trace("InsertPostTrialInvoice", externalID, zuoraAccountNumber, usageImportID, err) }()
	return t.d.InsertPostTrialInvoice(ctx, externalID, zuoraAccountNumber, usageImportID)
}

func (t traced) GetPostTrialInvoices(ctx context.Context) (pti []PostTrialInvoice, err error) {
	defer func() { t.trace("GetPostTrialInvoices", len(pti), err) }()
	return t.d.GetPostTrialInvoices(ctx)
}

func (t traced) DeletePostTrialInvoice(ctx context.Context, usageImportID string) (err error) {
	defer func() { t.trace("DeletePostTrialInvoice", usageImportID, err) }()
	return t.d.DeletePostTrialInvoice(ctx, usageImportID)
}

func (t traced) FindBillingAccountByTeamID(ctx context.Context, teamID string) (account *grpc.BillingAccount, err error) {
	defer func() { t.trace("FindBillingAccountByTeamID", teamID, account, err) }()
	return t.d.FindBillingAccountByTeamID(ctx, teamID)
}

func (t traced) SetTeamBillingAccountProvider(ctx context.Context, teamID, providerName string) (account *grpc.BillingAccount, err error) {
	defer func() { t.trace("SetTeamBillingAccountProvider", teamID, providerName, account, err) }()
	return t.d.SetTeamBillingAccountProvider(ctx, teamID, providerName)
}

func (t traced) Transaction(f func(DB) error) error {
	// We don't time transactions as they are only used in tests
	return t.d.Transaction(f)
}

func (t traced) Close(ctx context.Context) (err error) {
	defer func() { t.trace("Close", err) }()
	return t.d.Close(ctx)
}
