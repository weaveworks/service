package postgres

import (
	"context"
	"database/sql"
	"net/url"
	"strconv"
	"time"

	"github.com/Masterminds/squirrel"
	_ "github.com/lib/pq" // Import the postgres sql driver
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	_ "gopkg.in/mattes/migrate.v1/driver/postgres" // Import the postgres migrations driver
	"gopkg.in/mattes/migrate.v1/migrate"

	"github.com/weaveworks/service/common/dbwait"
	"github.com/weaveworks/service/users"
)

// DB is a postgres db, for dev and production, it implements db.DB
type DB struct {
	dbProxy
	squirrel.StatementBuilderType
	PasswordHashingCost int
}

type dbProxy interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) *sql.Row
	Prepare(query string) (*sql.Stmt, error)
}

// New creates a new postgres DB.
func New(databaseURI, migrationsDir string, passwordHashingCost int) (DB, error) {
	u, err := url.Parse(databaseURI)
	if err != nil {
		return DB{}, err
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
				return DB{}, errors.Wrapf(err, "parsing %s", k)
			}
			intOptions[k] = int(val)
		}
	}
	u.RawQuery = query.Encode()
	databaseURI = u.String()

	db, err := sql.Open("postgres", databaseURI)
	if err != nil {
		return DB{}, err
	}

	if err := dbwait.Wait(db); err != nil {
		return DB{}, errors.Wrap(err, "cannot establish db connection")
	}

	if migrationsDir != "" {
		log.Infof("Running Database Migrations...")
		if errs, ok := migrate.UpSync(databaseURI, migrationsDir); !ok {
			for _, err := range errs {
				log.Error(err)
			}
			return DB{}, errors.New("Database migrations failed")
		}
	}

	db.SetMaxOpenConns(intOptions["max_open_conns"])
	db.SetMaxIdleConns(intOptions["max_idle_conns"])

	return DB{
		dbProxy:              db,
		StatementBuilderType: statementBuilder(db),
		PasswordHashingCost:  passwordHashingCost,
	}, err
}

var statementBuilder = squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar).RunWith

// Now gives us the current time for Postgres. Postgres only stores times to
// the microsecond, so we pre-truncate times so tests will match. We also
// normalize to UTC, for sanity.
func (d DB) Now() time.Time {
	return time.Now().UTC().Truncate(time.Microsecond)
}

// Transaction runs the given function in a postgres transaction. If fn returns
// an error the txn will be rolled back.
func (d DB) Transaction(f func(DB) error) error {
	type beginner interface {
		Begin() (*sql.Tx, error)
	}
	if _, ok := d.dbProxy.(beginner); !ok {
		// Already in a nested transaction
		return f(d)
	}

	tx, err := d.dbProxy.(beginner).Begin()
	if err != nil {
		return err
	}
	err = f(DB{
		dbProxy:              tx,
		StatementBuilderType: statementBuilder(tx),
		PasswordHashingCost:  d.PasswordHashingCost,
	})
	if err != nil {
		// Rollback error is ignored as we already have one in progress
		if err2 := tx.Rollback(); err2 != nil {
			log.Warn("transaction rollback: %v (ignored)", err2)
		}
		return err
	}
	return tx.Commit()
}

// ListMemberships lists memberships list memberships
func (d DB) ListMemberships(_ context.Context) ([]users.Membership, error) {
	rows, err := d.dbProxy.Query(`
	SELECT
		memberships.user_id,
		memberships.organization_id
	FROM memberships
	WHERE memberships.deleted_at is null
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	memberships := []users.Membership{}
	for rows.Next() {
		membership := users.Membership{}
		if err := rows.Scan(
			&membership.UserID, &membership.OrganizationID,
		); err != nil {
			return nil, err
		}
		memberships = append(memberships, membership)
	}
	if rows.Err() != nil {
		return nil, err
	}
	return memberships, nil
}

// Close finishes using the db
func (d DB) Close(_ context.Context) error {
	if db, ok := d.dbProxy.(interface {
		Close() error
	}); ok {
		return db.Close()
	}
	return nil
}
