package postgres

import (
	"database/sql"
	"errors"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/Sirupsen/logrus"
	_ "github.com/lib/pq"                         // Import the postgres sql driver
	_ "github.com/mattes/migrate/driver/postgres" // Import the postgres migrations driver
	"github.com/mattes/migrate/migrate"
)

type pgDB struct {
	*sql.DB
	squirrel.StatementBuilderType
	PasswordHashingCost int
}

type queryRower interface {
	QueryRow(query string, args ...interface{}) *sql.Row
}

type execer interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
}

type execQueryRower interface {
	execer
	queryRower
}

// New creates a new postgres DB
func New(databaseURI, migrationsDir string, passwordHashingCost int) (*pgDB, error) {
	if migrationsDir != "" {
		logrus.Infof("Running Database Migrations...")
		if errs, ok := migrate.UpSync(databaseURI, migrationsDir); !ok {
			for _, err := range errs {
				logrus.Error(err)
			}
			return nil, errors.New("Database migrations failed")
		}
	}
	db, err := sql.Open("postgres", databaseURI)
	return &pgDB{
		DB:                   db,
		StatementBuilderType: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar).RunWith(db),
		PasswordHashingCost:  passwordHashingCost,
	}, err
}

// Now gives us the current time for Postgres. Postgres only stores times to
// the microsecond, so we pre-truncate times so tests will match. We also
// normalize to UTC, for sanity.
func (s pgDB) Now() time.Time {
	return time.Now().UTC().Truncate(time.Microsecond)
}

// Transaction runs the given function in a postgres transaction. If fn returns
// an error the txn will be rolled back.
func (s pgDB) Transaction(f func(*sql.Tx) error) error {
	tx, err := s.Begin()
	if err != nil {
		return err
	}
	if err = f(tx); err != nil {
		// Rollback error is ignored as we already have one in progress
		if err2 := tx.Rollback(); err2 != nil {
			logrus.Warn("transaction rollback: %v (ignored)", err2)
		}
		return err
	}
	return tx.Commit()
}

// Truncate clears all the data in pg. Should only be used in tests!
func (s pgDB) Truncate() error {
	return s.Transaction(func(tx *sql.Tx) error {
		return mustExec(
			tx,
			`truncate table traceable;`,
			`truncate table users;`,
			`truncate table logins;`,
			`truncate table organizations;`,
			`truncate table memberships;`,
		)
	})
}

func mustExec(e squirrel.Execer, queries ...string) error {
	for _, q := range queries {
		if _, err := e.Exec(q); err != nil {
			return err
		}
	}
	return nil
}
