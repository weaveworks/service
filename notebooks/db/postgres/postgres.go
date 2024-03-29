package postgres

import (
	"database/sql"
	"encoding/json"

	"github.com/pkg/errors"

	"github.com/Masterminds/squirrel"
	log "github.com/sirupsen/logrus"
	"github.com/weaveworks/service/notebooks"
	"gopkg.in/mattes/migrate.v1/migrate"

	uuid "github.com/satori/go.uuid"
	"github.com/weaveworks/service/common/dbwait"

	_ "github.com/lib/pq"                          // Import the postgres sql driver
	_ "gopkg.in/mattes/migrate.v1/driver/postgres" // Import the postgres migrations driver
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
		"version",
		"title",
		"entries",
		"query_end",
		"query_range",
		"trailing_now",
	).
		From("notebooks").
		Where(squirrel.Eq{"org_id": orgID}).
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
			&notebook.Version,
			&notebook.Title,
			&entriesBytes,
			&notebook.QueryEnd,
			&notebook.QueryRange,
			&notebook.TrailingNow,
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
func (d DB) CreateNotebook(notebook notebooks.Notebook) (string, error) {
	entriesBytes, err := json.Marshal(notebook.Entries)
	if err != nil {
		return "", err
	}

	newID := uuid.NewV4().String()
	newVersion := uuid.NewV4().String()
	_, err = d.Insert("notebooks").
		Columns(
			"id",
			"org_id",
			"created_by",
			"updated_by",
			"version",
			"title",
			"entries",
			"query_end",
			"query_range",
			"trailing_now",
		).
		Values(
			newID,
			notebook.OrgID,
			notebook.CreatedBy,
			notebook.UpdatedBy,
			newVersion,
			notebook.Title,
			entriesBytes,
			notebook.QueryEnd,
			notebook.QueryRange,
			notebook.TrailingNow,
		).
		Exec()

	return newID, err
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
		"version",
		"title",
		"entries",
		"query_end",
		"query_range",
		"trailing_now",
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
			&notebook.Version,
			&notebook.Title,
			&entriesBytes,
			&notebook.QueryEnd,
			&notebook.QueryRange,
			&notebook.TrailingNow,
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
func (d DB) UpdateNotebook(ID, orgID string, notebook notebooks.Notebook, version string) error {
	// Fetch the current notebook and check the version
	currentNotebook, err := d.GetNotebook(ID, orgID)
	if err != nil {
		// Matches how 'logging.With' would do it.
		log.WithField("orgID", orgID).Errorf("Error fetching current notebook: %v", err)
		return err
	}
	if version != currentNotebook.Version.String() {
		return notebooks.ErrNotebookVersionMismatch
	}

	entriesBytes, err := json.Marshal(notebook.Entries)
	if err != nil {
		return err
	}

	newVersion := uuid.NewV4()
	_, err = d.Update("notebooks").
		SetMap(
			map[string]interface{}{
				"updated_by":   notebook.UpdatedBy,
				"updated_at":   squirrel.Expr("now()"),
				"version":      newVersion,
				"title":        notebook.Title,
				"entries":      entriesBytes,
				"query_end":    notebook.QueryEnd,
				"query_range":  notebook.QueryRange,
				"trailing_now": notebook.TrailingNow,
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
			log.Warnf("transaction rollback: %v (ignored)", err2)
		}
		return err
	}
	return tx.Commit()
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
