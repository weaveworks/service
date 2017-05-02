package postgres

import (
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/Masterminds/squirrel"
	"github.com/Sirupsen/logrus"
	"github.com/mattes/migrate/migrate"
	"github.com/weaveworks/service/prom"

	_ "github.com/lib/pq"                         // Import the postgres sql driver
	_ "github.com/mattes/migrate/driver/postgres" // Import the postgres migrations driver
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
func New(uri, migrationsDir string) (DB, error) {
	if migrationsDir != "" {
		logrus.Infof("Running Database Migrations...")
		if errs, ok := migrate.UpSync(uri, migrationsDir); !ok {
			for _, err := range errs {
				logrus.Error(err)
			}
			return DB{}, errors.New("Database migrations failed")
		}
	}
	db, err := sql.Open("postgres", uri)
	return DB{
		dbProxy:              db,
		StatementBuilderType: statementBuilder(db),
	}, err
}

var statementBuilder = squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar).RunWith

// CreateNotebook returns all notebooks for instance
func (d DB) CreateNotebook(notebook prom.Notebook) error {
	entriesBytes, err := json.Marshal(notebook.Entries)
	if err != nil {
		return err
	}
	_, err = d.Insert("notebooks").
		Columns("id", "org_id", "title", "author_id", "updated_at", "entries").
		Values(notebook.ID, notebook.OrgID, notebook.Title, notebook.AuthorID, notebook.UpdatedAt, entriesBytes).
		Exec()

	return err
}

// GetAllNotebooks returns all notebooks for instance
func (d DB) GetAllNotebooks(orgID string) ([]prom.Notebook, error) {
	rows, err := d.Select("id", "org_id", "title", "author_id", "updated_at", "entries").
		From("notebooks").
		Where(squirrel.Eq{"org_id": orgID}).
		OrderBy("updated_at DESC").
		Query()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	notebooks := []prom.Notebook{}
	for rows.Next() {
		var notebook prom.Notebook
		var entriesBytes []byte
		err = rows.Scan(&notebook.ID, &notebook.OrgID, &notebook.Title, &notebook.AuthorID, &notebook.UpdatedAt, &entriesBytes)
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal(entriesBytes, &notebook.Entries)
		if err != nil {
			return nil, err
		}
		notebooks = append(notebooks, notebook)
	}

	return notebooks, nil
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
