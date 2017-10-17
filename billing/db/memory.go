package db

import (
	"context"
	"sync"
	"time"
)

// DB is an in-memory database for testing, and local development
type memory struct {
	mtx                        sync.RWMutex
	aggregates                 []Aggregate
	postTrialInvoices          map[string]PostTrialInvoice
	usageUploadsMaxAggregateID map[string]int // Maps uploaders to agg ID
}

// New creates a new in-memory database
func newMemory() *memory {
	return &memory{
		postTrialInvoices:          make(map[string]PostTrialInvoice),
		usageUploadsMaxAggregateID: make(map[string]int),
	}
}

func (db *memory) UpsertAggregates(ctx context.Context, aggregates []Aggregate) (err error) {
	db.mtx.Lock()
	defer db.mtx.Unlock()
	lastid := 0
	for _, a := range db.aggregates {
		if a.ID > lastid {
			lastid = a.ID
		}
	}
	for idx := range aggregates {
		lastid++
		aggregates[idx].ID = lastid
	}
	db.aggregates = append(db.aggregates, aggregates...)
	return nil
}

func (db *memory) GetAggregates(ctx context.Context, instanceID string, from, through time.Time) (as []Aggregate, err error) {
	return db.GetAggregatesAfter(ctx, instanceID, from, through, 0)
}

func (db *memory) GetAggregatesAfter(ctx context.Context, instanceID string, from, through time.Time, fromID int) (as []Aggregate, err error) {
	db.mtx.RLock()
	defer db.mtx.RUnlock()

	var result []Aggregate
	for _, a := range db.aggregates {
		if a.ID <= fromID {
			continue
		}
		if a.InstanceID != instanceID {
			continue
		}
		if a.BucketStart.Before(from) || a.BucketStart.After(through) {
			continue
		}
		result = append(result, a)
	}
	return result, nil
}

func (db *memory) InsertUsageUpload(ctx context.Context, uploader string, aggregateID int) (int64, error) {
	db.usageUploadsMaxAggregateID[uploader] = aggregateID
	return 1, nil
}

func (db *memory) DeleteUsageUpload(ctx context.Context, uploader string, aggregateID int64) error {
	delete(db.usageUploadsMaxAggregateID, uploader)
	return nil
}

func (db *memory) GetUsageUploadLargestAggregateID(ctx context.Context, uploader string) (int, error) {
	return db.usageUploadsMaxAggregateID[uploader], nil
}

func (db *memory) GetMonthSums(ctx context.Context, instanceIDs []string, from, through time.Time) (map[string][]Aggregate, error) {
	db.mtx.RLock()
	defer db.mtx.RUnlock()
	sums := map[string][]Aggregate{}
	for _, instanceID := range instanceIDs {
		aggs, err := db.GetAggregates(ctx, instanceID, from, through)
		if err != nil {
			return nil, err
		}
		var current time.Time
		sumsByAmountType := map[string]int64{}
		if len(aggs) > 0 {
			current = time.Date(aggs[0].BucketStart.Year(), aggs[0].BucketStart.Month(), 1, 0, 0, 0, 0, time.UTC)
			for _, agg := range aggs {
				if current.AddDate(0, 1, 0).Before(agg.BucketStart) {
					// Past the bucket, flush
					for amountType, value := range sumsByAmountType {
						sums[instanceID] = append(sums[instanceID], Aggregate{
							InstanceID:  instanceID,
							BucketStart: current,
							AmountType:  amountType,
							AmountValue: value,
						})
					}
					current = time.Date(agg.BucketStart.Year(), agg.BucketStart.Month(), 1, 0, 0, 0, 0, time.UTC)
					sumsByAmountType = map[string]int64{}
				}
				sumsByAmountType[agg.AmountType] += agg.AmountValue
			}
			// Flush final
			for amountType, value := range sumsByAmountType {
				sums[instanceID] = append(sums[instanceID], Aggregate{
					InstanceID:  instanceID,
					BucketStart: current,
					AmountType:  amountType,
					AmountValue: value,
				})
			}
		}
	}

	return sums, nil
}

func (db *memory) InsertPostTrialInvoice(ctx context.Context, externalID, zuoraAccountNumber, usageImportID string) (err error) {
	db.mtx.Lock()
	defer db.mtx.Unlock()
	invoice := PostTrialInvoice{
		ExternalID:         externalID,
		ZuoraAccountNumber: zuoraAccountNumber,
		UsageImportID:      usageImportID,
		CreatedAt:          time.Now().UTC(),
	}
	db.postTrialInvoices[usageImportID] = invoice
	return nil
}

func (db *memory) GetPostTrialInvoices(ctx context.Context) ([]PostTrialInvoice, error) {
	db.mtx.RLock()
	defer db.mtx.RUnlock()
	var invoices []PostTrialInvoice
	for _, invoice := range db.postTrialInvoices {
		invoices = append(invoices, invoice)
	}
	return invoices, nil
}

func (db *memory) DeletePostTrialInvoice(ctx context.Context, usageImportID string) (err error) {
	db.mtx.Lock()
	defer db.mtx.Unlock()
	delete(db.postTrialInvoices, usageImportID)
	return nil
}

func (db *memory) Transaction(f func(DB) error) error {
	return f(db)
}

func (db *memory) Close(ctx context.Context) (err error) {
	return nil
}
