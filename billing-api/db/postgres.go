package db

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/Masterminds/squirrel"
	pq "github.com/lib/pq" // Import the postgres sql driver
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/weaveworks/service/common/billing/grpc"
	"github.com/weaveworks/service/common/billing/provider"
	"github.com/weaveworks/service/common/dbwait"
	_ "gopkg.in/mattes/migrate.v1/driver/postgres" // Import the postgres migrations driver
	"gopkg.in/mattes/migrate.v1/migrate"
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

var aggregateColumns = []string{
	"aggregates.id",
	"aggregates.instance_id",
	"aggregates.bucket_start",
	"aggregates.amount_type",
	"aggregates.amount_value",
	"aggregates.created_at",
	"aggregates.upload_id",
}

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

	db, err := sql.Open("postgres", databaseURI)
	if err != nil {
		return nil, err
	}

	if err := dbwait.Wait(db); err != nil {
		return nil, errors.Wrap(err, "cannot establish db connection")
	}

	if migrationsDir != "" {
		log.Infof("Running Database Migrations...")
		if errs, ok := migrate.UpSync(databaseURI, migrationsDir); !ok {
			for _, err := range errs {
				log.Error(err)
			}
			return nil, errors.New("Database migrations failed")
		}
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
			log.Warnf("transaction rollback: %v (ignored)", err2)
		}
		return err
	}
	return tx.Commit()
}

func (d *postgres) InsertAggregates(ctx context.Context, aggregates []Aggregate) error {
	if len(aggregates) == 0 {
		return nil
	}
	insert := d.Insert(tableAggregates).
		Columns("instance_id", "bucket_start", "amount_type", "amount_value", "upload_id")

	for _, aggregate := range aggregates {
		var uploadID sql.NullInt64
		if aggregate.UploadID == 0 {
			uploadID = sql.NullInt64{}
		} else {
			uploadID = sql.NullInt64{Int64: aggregate.UploadID, Valid: true}
		}
		insert = insert.Values(
			aggregate.InstanceID, aggregate.BucketStart, aggregate.AmountType, aggregate.AmountValue, uploadID)
	}

	log.Debug(insert.ToSql())
	_, err := insert.Exec()
	return err
}

func (d *postgres) GetAggregates(ctx context.Context, instanceID string, from, through time.Time) (as []Aggregate, err error) {
	q := d.Select(aggregateColumns...).
		From(tableAggregates).
		Where(squirrel.Eq{"aggregates.instance_id": instanceID}).
		Where(squirrel.Lt{"aggregates.bucket_start": through}).
		OrderBy("aggregates.bucket_start asc", "aggregates.amount_type asc")
	if !from.IsZero() {
		q = q.Where(squirrel.GtOrEq{"aggregates.bucket_start": from})
	}
	return d.aggregateQueryScan(q)
}

func (d *postgres) GetAggregatesToUpload(ctx context.Context, instanceID string, from, through time.Time) ([]Aggregate, error) {
	q := d.Select(aggregateColumns...).
		From(tableAggregates).
		Where(squirrel.Eq{"aggregates.upload_id": nil}).
		Where(squirrel.Eq{"aggregates.instance_id": instanceID}).
		Where(squirrel.Lt{"aggregates.bucket_start": through}).
		Where(squirrel.GtOrEq{"aggregates.bucket_start": from}).
		OrderBy("aggregates.bucket_start asc", "aggregates.amount_type asc")
	return d.aggregateQueryScan(q)
}

func (d *postgres) GetAggregatesUploaded(ctx context.Context, uploadID int64) ([]Aggregate, error) {
	q := d.Select(aggregateColumns...).
		From(tableAggregates).
		Where(squirrel.Eq{"aggregates.upload_id": uploadID}).
		OrderBy("aggregates.bucket_start asc", "aggregates.amount_type asc")
	return d.aggregateQueryScan(q)
}

func (d *postgres) GetAggregatesFrom(ctx context.Context, instanceIDs []string, from time.Time) ([]Aggregate, error) {
	q := d.Select(aggregateColumns...).
		From(tableAggregates).
		Where(squirrel.Eq{"aggregates.instance_id": instanceIDs}).
		OrderBy("aggregates.bucket_start asc", "aggregates.amount_type asc")
	if !from.IsZero() {
		q = q.Where(squirrel.GtOrEq{"aggregates.bucket_start": from})
	}
	return d.aggregateQueryScan(q)
}

func (d *postgres) aggregateQueryScan(q squirrel.SelectBuilder) (as []Aggregate, err error) {
	rows, err := q.Query()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return d.scanAggregates(rows)
}

func (d postgres) scanAggregates(rows *sql.Rows) ([]Aggregate, error) {
	var aggregates []Aggregate
	var createdAt pq.NullTime
	var uploadID sql.NullInt64
	for rows.Next() {
		var aggregate Aggregate
		if err := rows.Scan(
			&aggregate.ID,
			&aggregate.InstanceID, &aggregate.BucketStart,
			&aggregate.AmountType, &aggregate.AmountValue,
			&createdAt, &uploadID,
		); err != nil {
			return nil, err
		}
		aggregate.CreatedAt = createdAt.Time
		aggregate.UploadID = uploadID.Int64
		aggregates = append(aggregates, aggregate)
	}
	err := rows.Err()
	if err != nil {
		return nil, err
	}
	return aggregates, nil
}

func (d *postgres) GetLatestUsageUpload(ctx context.Context, uploader string) (*UsageUpload, error) {
	q := d.Select("id", "uploader").
		From(tableUsageUploads).
		OrderBy("id desc").Limit(1)
	if uploader != "" {
		q = q.Where(squirrel.Eq{"uploader": uploader})
	}
	result := UsageUpload{}
	err := q.Scan(&result.ID, &result.Uploader)
	return &result, err
}

func (d *postgres) InsertUsageUpload(ctx context.Context, uploader string, aggregatesIDs []int) (int64, error) {
	var id int64
	err := d.QueryRow("INSERT INTO usage_uploads (max_aggregate_id, uploader) VALUES (-1, $1) RETURNING id", uploader).
		Scan(&id)
	if err != nil {
		return 0, err
	}
	_, err = d.Update(tableAggregates).Where(squirrel.Eq{"id": aggregatesIDs}).Set("upload_id", id).Query()
	if err != nil {
		d.DeleteUsageUpload(ctx, uploader, id)
		return id, err
	}
	return id, nil
}

func (d *postgres) DeleteUsageUpload(ctx context.Context, uploader string, uploadID int64) error {
	_, err := d.Delete(tableUsageUploads).Where(squirrel.Eq{"id": uploadID, "uploader": uploader}).Exec()
	if err != nil {
		return err
	}
	_, err = d.Update(tableAggregates).Where(squirrel.Eq{"upload_id": uploadID}).Set("upload_id", nil).Exec()
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

func (d postgres) FindBillingAccountByTeamID(ctx context.Context, teamID string) (*grpc.BillingAccount, error) {
	rows, err := d.billingAccounts().
		Join("billing_accounts_teams ON billing_accounts_teams.billing_account_id = billing_accounts.id").
		Where(squirrel.Eq{"billing_accounts_teams.team_id": teamID}).
		QueryContext(ctx)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	accounts, err := d.scanBillingAccounts(rows)
	if err != nil {
		return nil, err
	}
	// This should never happen, since enforced by the constraint in the DB's schema, but just in case:
	if len(accounts) > 1 {
		return nil, fmt.Errorf("more than one billing account for team with ID %s", teamID)
	}
	if len(accounts) == 0 {
		// Return a "null object" rather than nil and a "not found" error to avoid panics in the client.
		return &grpc.BillingAccount{}, nil
	}
	return accounts[0], nil
}

func (d postgres) billingAccounts() squirrel.SelectBuilder {
	return d.Select(
		"billing_accounts.id",
		"billing_accounts.created_at",
		"billing_accounts.deleted_at",
		"billing_accounts.billed_externally",
	).
		From("billing_accounts").
		Where("billing_accounts.deleted_at IS NULL").
		OrderBy("billing_accounts.created_at DESC")
}

func (d postgres) scanBillingAccounts(rows *sql.Rows) ([]*grpc.BillingAccount, error) {
	accounts := []*grpc.BillingAccount{}
	for rows.Next() {
		account, err := d.scanBillingAccount(rows)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, account)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return accounts, nil
}

func (d postgres) scanBillingAccount(row squirrel.RowScanner) (*grpc.BillingAccount, error) {
	a := &grpc.BillingAccount{}
	var deletedAt pq.NullTime
	var billedExternally bool
	if err := row.Scan(
		&a.ID,
		&a.CreatedAt,
		&deletedAt,
		&billedExternally,
	); err != nil {
		return nil, err
	}
	a.DeletedAt = deletedAt.Time
	if billedExternally {
		a.Provider = provider.External
	}
	return a, nil
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
