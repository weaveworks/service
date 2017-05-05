package postgres

import (
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/Masterminds/squirrel"
	"github.com/Sirupsen/logrus"
	"github.com/mattes/migrate/migrate"
	"github.com/weaveworks/service/notebooks"

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

// ListNotebooks returns all notebooks
func (d DB) ListNotebooks(orgID string) ([]notebooks.Notebook, error) {
	rows, err := d.Select(
		"id",
		"org_id",
		"created_by",
		"created_at",
		"updated_by",
		"updated_at",
		"title",
		"entries",
	).
		From("notebooks").
		Where(squirrel.Eq{"org_id": orgID}).
		OrderBy("updated_at DESC").
		Query()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ns := []notebooks.Notebook{}
	for rows.Next() {
		var notebook notebooks.Notebook
		var entriesBytes []byte
		err = rows.Scan(
			&notebook.ID,
			&notebook.OrgID,
			&notebook.CreatedBy,
			&notebook.CreatedAt,
			&notebook.UpdatedBy,
			&notebook.UpdatedAt,
			&notebook.Title,
			&entriesBytes,
		)
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal(entriesBytes, &notebook.Entries)
		if err != nil {
			return nil, err
		}
		ns = append(ns, notebook)
	}

	return ns, nil
}

// CreateNotebook creates a notebook
func (d DB) CreateNotebook(notebook notebooks.Notebook) error {
	entriesBytes, err := json.Marshal(notebook.Entries)
	if err != nil {
		return err
	}
	_, err = d.Insert("notebooks").
		Columns(
			"id",
			"org_id",
			"created_by",
			"updated_by",
			"title",
			"entries",
		).
		Values(
			notebook.ID,
			notebook.OrgID,
			notebook.CreatedBy,
			notebook.UpdatedBy,
			notebook.Title,
			entriesBytes,
		).
		Exec()

	return err
}

// GetNotebook returns the notebook with the same ID
func (d DB) GetNotebook(ID, orgID string) (notebooks.Notebook, error) {
	var notebook notebooks.Notebook
	var entriesBytes []byte

	err := d.Select(
		"id",
		"org_id",
		"created_by",
		"created_at",
		"updated_by",
		"updated_at",
		"title",
		"entries",
	).
		From("notebooks").
		Where(squirrel.Eq{"id": ID}, squirrel.Eq{"org_id": orgID}).
		QueryRow().
		Scan(
			&notebook.ID,
			&notebook.OrgID,
			&notebook.CreatedBy,
			&notebook.CreatedAt,
			&notebook.UpdatedBy,
			&notebook.UpdatedAt,
			&notebook.Title,
			&entriesBytes,
		)
	if err != nil {
		return notebooks.Notebook{}, err
	}

	err = json.Unmarshal(entriesBytes, &notebook.Entries)
	if err != nil {
		return notebooks.Notebook{}, err
	}

	return notebook, nil
}

// UpdateNotebook updates a notebook
func (d DB) UpdateNotebook(ID, orgID string, notebook notebooks.Notebook) error {
	entriesBytes, err := json.Marshal(notebook.Entries)
	if err != nil {
		return err
	}
	_, err = d.Update("notebooks").
		SetMap(
			map[string]interface{}{
				"updated_by": notebook.UpdatedBy,
				"updated_at": squirrel.Expr("now()"),
				"title":      notebook.Title,
				"entries":    entriesBytes,
			},
		).
		Where(squirrel.Eq{"id": ID}, squirrel.Eq{"org_id": orgID}).
		Exec()

	return err
}

// DeleteNotebook deletes a notebook
func (d DB) DeleteNotebook(ID, orgID string) error {
	_, err := d.Delete("notebooks").
		Where(squirrel.Eq{"id": ID}, squirrel.Eq{"org_id": orgID}).
		Exec()
	return err
}

// Close the database
func (d DB) Close() error {
	if db, ok := d.dbProxy.(interface {
		Close() error
	}); ok {
		return db.Close()
	}
	return nil
}
