package memory

import (
	"database/sql"

	"github.com/weaveworks/service/prom"
)

// DB is an in-memory database for testing, and local development
type DB struct {
	notebooks map[string][]prom.Notebook
	id        uint
}

// New creates a new in-memory database
func New(_, _ string) (*DB, error) {
	return &DB{
		notebooks: map[string][]prom.Notebook{},
		id:        0,
	}, nil
}

// GetAllNotebooks returns all notebooks for instance
func (d DB) GetAllNotebooks(orgID string) ([]prom.Notebook, error) {
	notebooks, ok := d.notebooks[orgID]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return notebooks, nil
}

// Close finishes using the db. Noop.
func (d *DB) Close() error {
	return nil
}
