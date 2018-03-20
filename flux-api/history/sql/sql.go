package sql

import (
	"database/sql"

	"github.com/Masterminds/squirrel"
	"github.com/jmoiron/sqlx"

	"github.com/weaveworks/service/flux-api/history"
)

// DB is a history DB that uses a SQL database.
type DB struct {
	driver *sqlx.DB
	squirrel.StatementBuilderType
}

// NewSQL creates a new DB.
func NewSQL(driver, datasource string) (history.DB, error) {
	db, err := sqlx.Open(driver, datasource)
	if err != nil {
		return nil, err
	}
	historyDB := &DB{
		driver:               db,
		StatementBuilderType: statementBuilder(db),
	}
	p := &pgDB{historyDB}
	return p, p.sanityCheck()
}

var statementBuilder = squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar).RunWith

// Query runs the given query against the DB.
func (db *DB) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return db.driver.Query(query, args...)
}

// Close closes the DB connection.
func (db *DB) Close() error {
	return db.driver.Close()
}
