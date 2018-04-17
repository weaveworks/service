package postgres

import (
	"database/sql"
	"strings"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"

	log "github.com/sirupsen/logrus"
	"github.com/weaveworks/service/common/dbwait"
	"github.com/weaveworks/service/notification-eventmanager/utils"
	"gopkg.in/mattes/migrate.v1/migrate"

	_ "github.com/lib/pq"                          // Import the postgres sql driver
	_ "gopkg.in/mattes/migrate.v1/driver/postgres" // Import the postgres migrations driver
)

var databaseRequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
	Namespace: "notification",
	Name:      "database_request_duration_seconds",
	Help:      "Time spent (in seconds) doing database requests.",
	Buckets:   prometheus.DefBuckets,
}, []string{"method", "status_code"})

// DB is a postgres db, for dev and production
type DB struct {
	client *utils.DB
}

func init() {
	prometheus.MustRegister(
		databaseRequestDuration,
	)
}

// New creates a new postgres DB
func New(uri, migrationsDir string) (DB, error) {
	if uri == "" {
		return DB{}, errors.New("Database URI is required")
	}

	db, err := sql.Open("postgres", uri)
	if err != nil {
		return DB{}, err
	}

	if err := dbwait.Wait(db); err != nil {
		return DB{}, errors.Wrap(err, "cannot establish db connection")
	}

	if migrationsDir != "" {
		log.Infof("Running Database Migrations...")
		if errs, ok := migrate.UpSync(uri, migrationsDir); !ok {
			for _, err := range errs {
				log.Error(err)
			}
			return DB{}, errors.New("Database migrations failed")
		}
	}
	d := utils.NewDB(db, databaseRequestDuration)

	return DB{client: d}, nil
}

// for each row returned from query, calls callback
func forEachRow(rows *sql.Rows, callback func(*sql.Rows) error) error {
	var err error
	for err == nil && rows.Next() {
		err = callback(rows)
	}
	if err == nil {
		err = rows.Err()
	}
	return err
}

// Executes the given function in a transaction, and rolls back or commits depending on if the function errors.
// Ignores errors from rollback.
func (d DB) withTx(method string, f func(tx *utils.Tx) error) error {
	tx, err := d.client.Begin(method)
	if err != nil {
		return err
	}
	err = f(tx)
	if err == nil {
		return tx.Commit()
	}
	_ = tx.Rollback()

	if strings.Contains(err.Error(), "pq: duplicate key value violates unique constraint \"instances_initialized_pkey\"") {
		// if error is pq: duplicate key value violates unique constraint "instances_initialized_pkey"
		// instance already initialized, ignore this error
		return nil
	}

	return err
}
