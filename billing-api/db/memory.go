package db

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/weaveworks/service/common/billing/grpc"
)

// DB is an in-memory database for testing, and local development
type memory struct {
	mtx                        sync.RWMutex
	aggregatesSet              map[int]Aggregate // To allow for O(1) presence checks.
	uploads                    []UsageUpload
	postTrialInvoices          map[string]PostTrialInvoice
	usageUploadsMaxAggregateID map[string]int // Maps uploaders to agg ID
	billingAccountsByTeamID    map[string]*grpc.BillingAccount
}

// New creates a new in-memory database
func newMemory() *memory {
	return &memory{
		aggregatesSet:              make(map[int]Aggregate),
		postTrialInvoices:          make(map[string]PostTrialInvoice),
		usageUploadsMaxAggregateID: make(map[string]int),
		billingAccountsByTeamID:    make(map[string]*grpc.BillingAccount),
		uploads:                    []UsageUpload{},
	}
}

func (db *memory) InsertAggregates(ctx context.Context, aggregates []Aggregate) (err error) {
	db.mtx.Lock()
	defer db.mtx.Unlock()
	lastid := 0
	now := time.Now()
	for _, a := range db.aggregatesSet {
		if a.ID > lastid {
			lastid = a.ID
		}
	}
	for _, a := range aggregates {
		lastid++
		a.ID = lastid
		if _, ok := db.aggregatesSet[a.ID]; ok {
			return fmt.Errorf("duplicate aggregate: %v", a)
		}
		a.CreatedAt = now
		db.aggregatesSet[a.ID] = a
	}
	return nil
}

func sortAggregates(aggs []Aggregate) {
	sort.Slice(aggs, func(i, j int) bool { return aggs[i].ID < aggs[j].ID })
}

func (db *memory) GetAggregates(ctx context.Context, instanceID string, from, through time.Time) (as []Aggregate, err error) {
	db.mtx.RLock()
	defer db.mtx.RUnlock()

	var result []Aggregate
	for _, a := range db.aggregatesSet {
		if a.BucketStart.Before(from) || a.BucketStart.After(through) {
			continue
		}
		if a.InstanceID != instanceID {
			continue
		}
		result = append(result, a)
	}
	sortAggregates(result)
	return result, nil
}

func (db *memory) GetAggregatesToUpload(ctx context.Context, instanceID string, from, through time.Time) (as []Aggregate, err error) {
	db.mtx.RLock()
	defer db.mtx.RUnlock()
	var result []Aggregate
	all, err := db.GetAggregates(ctx, instanceID, from, through)
	if err != nil {
		return nil, err
	}
	for _, a := range all {
		if a.UploadID != 0 {
			continue
		}
		result = append(result, a)
	}
	return result, nil
}

func (db *memory) GetAggregatesUploaded(ctx context.Context, uploadID int64) (as []Aggregate, err error) {
	db.mtx.RLock()
	defer db.mtx.RUnlock()

	var result []Aggregate
	for _, a := range db.aggregatesSet {
		if a.UploadID != uploadID {
			continue
		}
		result = append(result, a)
	}
	sortAggregates(result)
	return result, nil
}

func (db *memory) GetAggregatesFrom(ctx context.Context, instanceIDs []string, from time.Time) ([]Aggregate, error) {
	db.mtx.RLock()
	defer db.mtx.RUnlock()

	idsSet := make(map[string]struct{}, len(instanceIDs))
	for _, instanceID := range instanceIDs {
		idsSet[instanceID] = struct{}{}
	}

	var result []Aggregate
	for _, a := range db.aggregatesSet {
		if _, ok := idsSet[a.InstanceID]; !ok {
			continue
		}
		if a.BucketStart.Before(from) {
			continue
		}
		result = append(result, a)
	}
	sortAggregates(result)
	return result, nil
}

func (db *memory) InsertUsageUpload(ctx context.Context, uploader string, aggregateIDs []int) (int64, error) {
	db.mtx.RLock()
	defer db.mtx.RUnlock()

	uploadID := int64(len(db.uploads) + 1) // for this in-memory DB we're using 0 as a proxy for a DB null
	db.uploads = append(db.uploads, UsageUpload{ID: uploadID})
	for _, id := range aggregateIDs {
		agg := db.aggregatesSet[id]
		agg.UploadID = uploadID
		db.aggregatesSet[id] = agg
	}
	return uploadID, nil
}

func (db *memory) DeleteUsageUpload(ctx context.Context, uploader string, aggregateID int64) error {
	delete(db.usageUploadsMaxAggregateID, uploader)
	for id := range db.aggregatesSet {
		agg := db.aggregatesSet[id]
		agg.UploadID = 0
		db.aggregatesSet[id] = agg
	}
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

func (db *memory) FindBillingAccountByTeamID(ctx context.Context, teamID string) (*grpc.BillingAccount, error) {
	db.mtx.Lock()
	defer db.mtx.Unlock()
	return db.billingAccountsByTeamID[teamID], nil
}

func (db *memory) Transaction(f func(DB) error) error {
	return f(db)
}

func (db *memory) Close(ctx context.Context) (err error) {
	return nil
}
