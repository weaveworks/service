package postgres

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/Sirupsen/logrus"
	_ "github.com/lib/pq"                         // Import the postgres sql driver
	_ "github.com/mattes/migrate/driver/postgres" // Import the postgres migrations driver
	"github.com/mattes/migrate/migrate"
	"github.com/weaveworks/service/configs"
)

// DB is a postgres db, for dev and production
type DB struct {
	dbProxy
	squirrel.StatementBuilderType
}

type dbProxy interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) *sql.Row
	Prepare(query string) (*sql.Stmt, error)
}

// New creates a new postgres DB
func New(databaseURI, migrationsDir string) (DB, error) {
	if migrationsDir != "" {
		logrus.Infof("Running Database Migrations...")
		if errs, ok := migrate.UpSync(databaseURI, migrationsDir); !ok {
			for _, err := range errs {
				logrus.Error(err)
			}
			return DB{}, errors.New("Database migrations failed")
		}
	}
	db, err := sql.Open("postgres", databaseURI)
	return DB{
		dbProxy:              db,
		StatementBuilderType: statementBuilder(db),
	}, err
}

var statementBuilder = squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar).RunWith

func configMatches(id, entityType, subsystem string) squirrel.Eq {
	return squirrel.Eq{
		"deleted_at": nil,
		"type":       entityType,
		"subsystem":  subsystem,
		"id":         id,
	}
}

func (d DB) findConfig(id, entityType, subsystem string) (configs.Config, error) {
	var cfg configs.Config
	var cfgBytes []byte
	// XXX: tests for deleted at
	err := d.Select("config").
		From("configs").
		Where(configMatches(id, entityType, subsystem)).QueryRow().Scan(&cfgBytes)
	if err != nil {
		return cfg, err
	}
	err = json.Unmarshal(cfgBytes, &cfg)
	return cfg, err
}

// GetUserConfig gets a user's configuration.
func (d DB) GetUserConfig(userID configs.UserID, subsystem configs.Subsystem) (configs.Config, error) {
	// XXX: constant for type
	return d.findConfig(string(userID), "user", string(subsystem))
}

// SetUserConfig sets a user's configuration.
func (d DB) SetUserConfig(userID configs.UserID, subsystem configs.Subsystem, cfg configs.Config) error {
	cfgBytes, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	return d.Transaction(func(tx DB) error {
		_, err := d.findConfig(string(userID), "user", string(subsystem))
		if err == sql.ErrNoRows {
			_, err := d.Insert("configs").
				Columns("id", "type", "subsystem", "config").
				Values(string(userID), "user", string(subsystem), cfgBytes).
				Exec()
			return err
		}
		_, err = d.Update("configs").
			Where(configMatches(string(userID), "user", string(subsystem))).
			Set("config", cfgBytes).
			Exec()
		return err
	})
}

// GetOrgConfig gets a org's configuration.
func (d DB) GetOrgConfig(orgID configs.OrgID, subsystem configs.Subsystem) (configs.Config, error) {
	// XXX: constant for type
	return d.findConfig(string(orgID), "org", string(subsystem))
}

// SetOrgConfig sets a org's configuration.
func (d DB) SetOrgConfig(orgID configs.OrgID, subsystem configs.Subsystem, cfg configs.Config) error {
	cfgBytes, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	return d.Transaction(func(tx DB) error {
		_, err := d.findConfig(string(orgID), "org", string(subsystem))
		if err == sql.ErrNoRows {
			_, err := d.Insert("configs").
				Columns("id", "type", "subsystem", "config").
				Values(string(orgID), "org", string(subsystem), cfgBytes).
				Exec()
			return err
		}
		_, err = d.Update("configs").
			Where(configMatches(string(orgID), "org", string(subsystem))).
			Set("config", cfgBytes).
			Exec()
		return err
	})
}

// Now gives us the current time for Postgres. Postgres only stores times to
// the microsecond, so we pre-truncate times so tests will match. We also
// normalize to UTC, for sanity.
func (d DB) Now() time.Time {
	return time.Now().UTC().Truncate(time.Microsecond)
}

// Transaction runs the given function in a postgres transaction. If fn returns
// an error the txn will be rolled back.
func (d DB) Transaction(f func(DB) error) error {
	if _, ok := d.dbProxy.(*sql.Tx); ok {
		// Already in a nested transaction
		return f(d)
	}

	tx, err := d.dbProxy.(*sql.DB).Begin()
	if err != nil {
		return err
	}
	err = f(DB{
		dbProxy:              tx,
		StatementBuilderType: statementBuilder(tx),
	})
	if err != nil {
		// Rollback error is ignored as we already have one in progress
		if err2 := tx.Rollback(); err2 != nil {
			logrus.Warn("transaction rollback: %v (ignored)", err2)
		}
		return err
	}
	return tx.Commit()
}

// Close finishes using the db
func (d DB) Close() error {
	if db, ok := d.dbProxy.(interface {
		Close() error
	}); ok {
		return db.Close()
	}
	return nil
}
