package db

import (
	"context"
	"flag"
	"fmt"
	"net/url"
	"time"
)

// Aggregate represents a database row in table `aggregates`.
type Aggregate struct {
	ID          int
	InstanceID  string
	BucketStart time.Time
	AmountType  string
	AmountValue int64
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
	UpsertAggregates(ctx context.Context, aggregates []Aggregate) error
	GetAggregates(ctx context.Context, instanceID string, from, through time.Time) ([]Aggregate, error)
	// GetAggregatesAfter returns all aggregates with an ID greater than fromID. It also requires a `through` time
	// and supports an optional `from` time.
	GetAggregatesAfter(ctx context.Context, instanceID string, from, through time.Time, fromID int) ([]Aggregate, error)

	// GetUsageUploadLargestAggregateID returns the largest aggregate ID that we have uploaded.
	GetUsageUploadLargestAggregateID(ctx context.Context) (int, error)
	// InsertUsageUpload records that we just uploaded all aggregates up to the given ID.
	InsertUsageUpload(ctx context.Context, maxAggregateID int) (int64, error)
	// DeleteUsageUpload removes our previously recorded upload after it failed.
	DeleteUsageUpload(ctx context.Context, uploadID int64) error

	GetMonthSums(ctx context.Context, instanceIDs []string, from, through time.Time) (map[string][]Aggregate, error)

	InsertPostTrialInvoice(ctx context.Context, externalID, zuoraAccountNumber, usageImportID string) error
	GetPostTrialInvoices(ctx context.Context) ([]PostTrialInvoice, error)
	DeletePostTrialInvoice(ctx context.Context, usageImportID string) error

	// Transaction runs the given function in a transaction. If fn returns
	// an error the txn will be rolled back.
	Transaction(f func(DB) error) error

	Close(ctx context.Context) error
}

// Config contains database settings.
type Config struct {
	DatabaseURI   string
	MigrationsDir string
}

// RegisterFlags registers configuration variables.
func (cfg *Config) RegisterFlags(f *flag.FlagSet) {
	f.StringVar(&cfg.DatabaseURI, "db.uri", "postgres://postgres@billing-db/billing?sslmode=disable", "Database to use.")
	f.StringVar(&cfg.MigrationsDir, "db.migrations", "/migrations", "Migrations directory.")
}

// New creates a new database from the URI
func New(cfg Config) (DB, error) {
	u, err := url.Parse(cfg.DatabaseURI)
	if err != nil {
		return nil, err
	}
	var d DB
	switch u.Scheme {
	case "memory":
		d = newMemory()
	case "postgres":
		d, err = newPostgres(cfg.DatabaseURI, cfg.MigrationsDir)
	default:
		return nil, fmt.Errorf("Unknown database type: %s", u.Scheme)
	}
	if err != nil {
		return nil, err
	}
	return traced{timed{d}}, nil
}
