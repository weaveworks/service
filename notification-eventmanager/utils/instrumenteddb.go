package utils

import (
	"context"
	"database/sql"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/weaveworks/common/instrument"
)

type wrappedDbOrTx interface {
	Query(string, ...interface{}) (*sql.Rows, error)
	QueryRow(string, ...interface{}) *sql.Row
	Exec(string, ...interface{}) (sql.Result, error)
}

// To generalize all the non-Begin() functions to wrapped Txes as well, we have this general struct
// that we embed in the more specific DB struct below.
type dbOrTx struct {
	collector     *instrument.HistogramCollector
	wrappedDbOrTx wrappedDbOrTx
}

// DB wraps a sql.DB object with modified versions of Query(), QueryRow(), Exec() and Begin()
// which take a method name (ie. an identifier for the operation you're doing) and log timing information
// as a prometheus metric (and an opentracing span if configured).
// Transactions created by Begin() are also wrapped in a similarly wrapped instrumented Tx object.
// All methods are identical to their wrapped counterparts, except with an extra first arg "method" as string.
// Note that not all methods are covered, only the ones listed above.
type DB struct {
	dbOrTx
	wrappedDb *sql.DB
}

// NewDB creates a new wrapped DB object given a DB object to wrap and a registered prometheus histogram metric
func NewDB(wrappedDb *sql.DB, metric *prometheus.HistogramVec) *DB {
	// Note that we store wrapped in two places, one as a concrete type so we can call .Begin() later,
	// the other as a general wrappedDbOrTx so we can have general implementations of Query(), QueryRow(), Exec()
	return &DB{
		dbOrTx: dbOrTx{
			collector:     instrument.NewHistogramCollector(metric),
			wrappedDbOrTx: wrappedDb,
		},
		wrappedDb: wrappedDb,
	}
}

func (db *dbOrTx) timed(method string, toStatusCode func(error) string, f func() error) error {
	return instrument.CollectedRequest(
		context.TODO(),
		method,
		db.collector,
		toStatusCode,
		func(_ context.Context) error {
			return f()
		},
	)
}

// Query wraps sql.DB.Query(), with a metric status 500 on error and 200 otherwise
func (db *dbOrTx) Query(method string, query string, args ...interface{}) (result *sql.Rows, err error) {
	db.timed(method, nil, func() error {
		result, err = db.wrappedDbOrTx.Query(query, args...)
		return err
	})
	return
}

// QueryRow wraps sql.DB.QueryRow(), with a metric status 200 (since we can't detect errors without doing a row.Scan)
func (db *dbOrTx) QueryRow(method string, query string, args ...interface{}) (result *sql.Row) {
	db.timed(method, nil, func() error {
		result = db.wrappedDbOrTx.QueryRow(query, args...)
		return nil
	})
	return
}

// Exec wraps sql.DB.Exec(), with a metric status 500 on error and 200 otherwise
func (db *dbOrTx) Exec(method string, query string, args ...interface{}) (result sql.Result, err error) {
	db.timed(method, nil, func() error {
		result, err = db.wrappedDbOrTx.Exec(query, args...)
		return err
	})
	return
}

// Tx represents a wrapped sql.Tx object and is obtained from DB.Begin()
type Tx struct {
	// inherits all the normal .Query(), .QueryRow(), .Exec() from the inner dbOrTx struct
	dbOrTx
	// needed for Tx-specific calls .Commit(), .Rollback()
	wrappedTx *sql.Tx
	method    string
	start     time.Time
}

// Begin wraps sql.DB.Begin(), returning a wrapped transaction object. The entire transaction time will be logged
// under the method given in this call, with individual queries during the transaction logged under their own methods.
// For example, if you did a tx = db.Begin("my_tx"), then tx.Query("foo", ...) which took 30ms,
// then tx.Exec("bar") which took 20ms, then committed the transaction, then the final result may show that method "foo"
// took 30ms, method "bar" took 20ms, and method "my_tx" took 50ms.
// The overall transaction will indicate a status of 200 on commit, 400 on rollback or 500 on error (including commit conflict).
func (db *DB) Begin(method string) (*Tx, error) {
	start := time.Now()
	db.collector.Before(method, start)
	wrappedTx, err := db.wrappedDb.Begin()
	if err != nil {
		db.collector.After(method, "500", start)
		return nil, err
	}
	return &Tx{
		dbOrTx: dbOrTx{
			collector:     db.collector,
			wrappedDbOrTx: wrappedTx,
		},
		wrappedTx: wrappedTx,
		method:    method,
		start:     start,
	}, nil
}

// Commit wraps Tx.Commit() and finishes the commit with a 200 on success or 500 on error
func (tx *Tx) Commit() error {
	err := tx.wrappedTx.Commit()
	code := "200"
	if err != nil {
		code = "500"
	}
	tx.collector.After(tx.method, code, tx.start)
	return err
}

// Rollback wraps Tx.Rollback() and finishes the commit with a 400 on success or 500 on error
func (tx *Tx) Rollback() error {
	err := tx.wrappedTx.Rollback()
	code := "400"
	if err != nil {
		code = "500"
	}
	tx.collector.After(tx.method, code, tx.start)
	return err
}
