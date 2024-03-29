package db

import (
	"context"
	"fmt"
	"time"

	"github.com/weaveworks/service/common/billing/grpc"
	"github.com/weaveworks/service/common/dbconfig"
)

// Aggregate represents a database row in table `aggregates`.
type Aggregate struct {
	ID          int
	InstanceID  string
	BucketStart time.Time
	AmountType  string
	AmountValue int64
	CreatedAt   time.Time
	UploadID    int64
}

// UsageUpload represents a database row in table `usage_uploads`.
type UsageUpload struct {
	ID       int64
	Uploader string
}

// PostTrialInvoice represents a database row in table `post_trial_invoices`.
type PostTrialInvoice struct {
	ExternalID         string
	ZuoraAccountNumber string
	UsageImportID      string
	CreatedAt          time.Time
}

// DB is the interface for the database.
type DB interface {
	InsertAggregates(ctx context.Context, aggregates []Aggregate) error
	// GetAggregates returns all aggregates. It also requires a `through` time and supports an optional `from` time.
	GetAggregates(ctx context.Context, instanceID string, from, through time.Time) ([]Aggregate, error)
	// GetAggregatesToUpload returns all aggregates which have not yet been uploaded. It also requires a `through` and `from` time.
	GetAggregatesToUpload(ctx context.Context, instanceID string, from, through time.Time) ([]Aggregate, error)
	// GetAggregatesUploaded returns all aggregates which assigned to a given upload.
	GetAggregatesUploaded(ctx context.Context, uploadID int64) ([]Aggregate, error)
	// GetAggregatesFrom returns all aggregates for the provided instance IDs from the provided time.
	GetAggregatesFrom(ctx context.Context, instanceIDs []string, from time.Time) ([]Aggregate, error)

	// InsertUsageUpload records that we just uploaded all aggregates up to the given ID.
	InsertUsageUpload(ctx context.Context, uploader string, aggregateIDs []int) (int64, error)
	// DeleteUsageUpload removes our previously recorded upload after it failed.
	DeleteUsageUpload(ctx context.Context, uploader string, uploadID int64) error
	// GetLatestUsageUpload finds the latest usage upload, optionally matching the given uploader name
	GetLatestUsageUpload(ctx context.Context, uploader string) (*UsageUpload, error)

	GetMonthSums(ctx context.Context, instanceIDs []string, from, through time.Time) (map[string][]Aggregate, error)

	InsertPostTrialInvoice(ctx context.Context, externalID, zuoraAccountNumber, usageImportID string) error
	GetPostTrialInvoices(ctx context.Context) ([]PostTrialInvoice, error)
	DeletePostTrialInvoice(ctx context.Context, usageImportID string) error

	// FindBillingAccountByTeamID looks for the billing account of
	// a team. If it cannot find one, will return an empty struct and no error.
	FindBillingAccountByTeamID(ctx context.Context, teamID string) (*grpc.BillingAccount, error)
	// SetTeamBillingAccountProvider makes sure a team has a billing
	// account reflecting the given provider name.
	SetTeamBillingAccountProvider(ctx context.Context, teamID, providerName string) (*grpc.BillingAccount, error)

	// Transaction runs the given function in a transaction. If fn returns
	// an error the txn will be rolled back.
	Transaction(f func(DB) error) error

	Close(ctx context.Context) error
}

// New creates a new database from the URI
func New(cfg dbconfig.Config) (DB, error) {
	scheme, dataSourceName, migrationsDir, err := cfg.Parameters()
	if err != nil {
		return nil, err
	}
	var d DB
	switch scheme {
	case "memory":
		d = newMemory()
	case "postgres":
		d, err = newPostgres(dataSourceName, migrationsDir)
	default:
		return nil, fmt.Errorf("Unknown database type: %s", scheme)
	}
	if err != nil {
		return nil, err
	}
	return traced{timed{d}}, nil
}
