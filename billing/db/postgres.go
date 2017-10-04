package db

import (
	"context"
	"database/sql"
	"net/url"
	"strconv"
	"time"

	"github.com/Masterminds/squirrel"
	log "github.com/Sirupsen/logrus"
	_ "github.com/lib/pq"                         // Import the postgres sql driver
	_ "github.com/mattes/migrate/driver/postgres" // Import the postgres migrations driver
	"github.com/mattes/migrate/migrate"
	"github.com/pkg/errors"
)

// postgres represents a connection to the database.
type postgres struct {
	dbProxy
	squirrel.StatementBuilderType
}

type dbProxy interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) *sql.Row
	Prepare(query string) (*sql.Stmt, error)
}

const (
	tableAggregates        = "aggregates"
	tableUsageUploads      = "usage_uploads"
	tablePostTrialInvoices = "post_trial_invoices"
)

// newPostgres creates a database connection.
func newPostgres(databaseURI, migrationsDir string) (*postgres, error) {
	u, err := url.Parse(databaseURI)
	if err != nil {
		return nil, err
	}
	intOptions := map[string]int{
		"max_open_conns": 0,
		"max_idle_conns": 0,
	}
	query := u.Query()
	for k := range intOptions {
		if valStr := query.Get(k); valStr != "" {
			query.Del(k) // Delete these options so lib/pq doesn't panic
			val, err := strconv.ParseInt(valStr, 10, 32)
			if err != nil {
				return nil, errors.Wrapf(err, "parsing %s", k)
			}
			intOptions[k] = int(val)
		}
	}
	u.RawQuery = query.Encode()
	databaseURI = u.String()

	if migrationsDir != "" {
		log.Infof("Running Database Migrations...")
		if errs, ok := migrate.UpSync(databaseURI, migrationsDir); !ok {
			for _, err := range errs {
				log.Error(err)
			}
			return nil, errors.New("Database migrations failed")
		}
	}
	db, err := sql.Open("postgres", databaseURI)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(intOptions["max_open_conns"])
	db.SetMaxIdleConns(intOptions["max_idle_conns"])

	return &postgres{
		dbProxy:              db,
		StatementBuilderType: statementBuilder(db),
	}, nil
}

var statementBuilder = squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar).RunWith

func (d *postgres) Transaction(f func(DB) error) error {
	if _, ok := d.dbProxy.(*sql.Tx); ok {
		// Already in a nested transaction
		return f(d)
	}

	tx, err := d.dbProxy.(*sql.DB).Begin()
	if err != nil {
		return err
	}
	err = f(&postgres{dbProxy: tx, StatementBuilderType: statementBuilder(tx)})
	if err != nil {
		// Rollback error is ignored as we already have one in progress
		if err2 := tx.Rollback(); err2 != nil {
			log.Warn("transaction rollback: %v (ignored)", err2)
		}
		return err
	}
	return tx.Commit()
}

func (d *postgres) UpsertAggregates(ctx context.Context, aggregates []Aggregate) error {
	insert := d.Insert(tableAggregates).
		Columns("instance_id", "bucket_start", "amount_type", "amount_value")

	for _, aggregate := range aggregates {
		insert = insert.Values(aggregate.InstanceID, aggregate.BucketStart, aggregate.AmountType, aggregate.AmountValue)
	}

	insert = insert.Suffix("ON CONFLICT (instance_id, bucket_start, amount_type) DO UPDATE SET amount_value=EXCLUDED.amount_value")
	log.Debug(insert.ToSql())
	_, err := insert.Exec()
	return err
}

func (d *postgres) GetAggregates(ctx context.Context, instanceID string, from, through time.Time) (as []Aggregate, err error) {
	return d.GetAggregatesAfter(ctx, instanceID, from, through, 0)
}

func (d *postgres) GetAggregatesAfter(ctx context.Context, instanceID string, from, through time.Time, lowerAggregatesID int) ([]Aggregate, error) {
	q := d.Select(
		"aggregates.id",
		"aggregates.instance_id",
		"aggregates.bucket_start",
		"aggregates.amount_type",
		"aggregates.amount_value",
	).
		From(tableAggregates).
		Where(squirrel.Gt{"aggregates.id": lowerAggregatesID}).
		Where(squirrel.Eq{"aggregates.instance_id": instanceID}).
		Where(squirrel.Lt{"aggregates.bucket_start": through}).
		OrderBy("aggregates.bucket_start asc", "aggregates.amount_type asc")
	if !from.IsZero() {
		q = q.Where(squirrel.GtOrEq{"aggregates.bucket_start": from})
	}
	rows, err := q.Query()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var aggregates []Aggregate
	for rows.Next() {
		var aggregate Aggregate
		if err := rows.Scan(
			&aggregate.ID,
			&aggregate.InstanceID, &aggregate.BucketStart,
			&aggregate.AmountType, &aggregate.AmountValue,
		); err != nil {
			return nil, err
		}
		aggregates = append(aggregates, aggregate)
	}
	if rows.Err() != nil {
		return nil, err
	}
	return aggregates, nil
}

func (d *postgres) GetUsageUploadLargestAggregateID(ctx context.Context) (int, error) {
	var max int
	err := d.QueryRow("SELECT MAX(max_aggregate_id) FROM usage_uploads").Scan(&max)
	if err != nil {
		return 0, err
	}
	return max, nil
}

func (d *postgres) InsertUsageUpload(ctx context.Context, aggregatesID int) (int64, error) {
	var id int64
	err := d.QueryRow("INSERT INTO usage_uploads (max_aggregate_id) VALUES ($1) RETURNING id", aggregatesID).
		Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (d *postgres) DeleteUsageUpload(ctx context.Context, uploadID int64) error {
	_, err := d.Delete(tableUsageUploads).Where(squirrel.Eq{"id": uploadID}).Exec()
	return err
}

func (d *postgres) GetMonthSums(ctx context.Context, instanceIDs []string, from, through time.Time) (map[string][]Aggregate, error) {
	rows, err := d.Select(
		"aggregates.instance_id",
		"extract(year from aggregates.bucket_start) as year",
		"extract(month from aggregates.bucket_start) as month",
		"aggregates.amount_type",
		"sum(aggregates.amount_value)",
	).
		From(tableAggregates).
		Where(squirrel.Eq{"aggregates.instance_id": instanceIDs}).
		Where(squirrel.GtOrEq{"aggregates.bucket_start": from}).
		Where(squirrel.Lt{"aggregates.bucket_start": through}).
		GroupBy("aggregates.instance_id", "year", "month", "aggregates.amount_type").
		OrderBy("year asc", "month asc", "aggregates.instance_id asc", "aggregates.amount_type asc").
		Query()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	aggregates := map[string][]Aggregate{}
	for rows.Next() {
		var (
			aggregate Aggregate
			month     time.Month
			year      int
		)
		if err := rows.Scan(
			&aggregate.InstanceID,
			&year,
			&month,
			&aggregate.AmountType,
			&aggregate.AmountValue,
		); err != nil {
			return nil, err
		}
		aggregate.BucketStart = time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
		aggregates[aggregate.InstanceID] = append(aggregates[aggregate.InstanceID], aggregate)
	}
	if rows.Err() != nil {
		return nil, err
	}
	return aggregates, nil
}

func (d *postgres) InsertPostTrialInvoice(ctx context.Context, externalID, zuoraAccountNumber, usageImportID string) error {
	insert := d.Insert(tablePostTrialInvoices).
		Columns("external_id", "zuora_account_number", "usage_import_id")
	insert = insert.Values(externalID, zuoraAccountNumber, usageImportID)
	log.Debug(insert.ToSql())
	_, err := insert.Exec()
	return err
}

func (d *postgres) GetPostTrialInvoices(ctx context.Context) ([]PostTrialInvoice, error) {
	rows, err := d.Select(
		"external_id",
		"zuora_account_number",
		"usage_import_id",
		"created_at",
	).
		From(tablePostTrialInvoices).
		OrderBy("created_at").
		Query()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var postTrialInvoices []PostTrialInvoice
	for rows.Next() {
		var postTrialInvoice PostTrialInvoice
		if err := rows.Scan(
			&postTrialInvoice.ExternalID,
			&postTrialInvoice.ZuoraAccountNumber,
			&postTrialInvoice.UsageImportID,
			&postTrialInvoice.CreatedAt,
		); err != nil {
			return nil, err
		}
		postTrialInvoices = append(postTrialInvoices, postTrialInvoice)
	}
	if rows.Err() != nil {
		return nil, err
	}
	return postTrialInvoices, nil
}

func (d *postgres) DeletePostTrialInvoice(ctx context.Context, usageImportID string) error {
	_, err := d.Delete(tablePostTrialInvoices).
		Where(squirrel.Eq{"usage_import_id": usageImportID}).
		Exec()
	return err
}

// Close finishes using the db
func (d *postgres) Close(_ context.Context) error {
	if db, ok := d.dbProxy.(interface {
		Close() error
	}); ok {
		return db.Close()
	}
	return nil
}
