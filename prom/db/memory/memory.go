package memory

import (
	"github.com/weaveworks/service/prom"
)

var currentID = 1

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

// CreateNotebook creates a notebook for the instance
func (d DB) CreateNotebook(notebook prom.Notebook) error {
	notebooks, ok := d.notebooks[notebook.OrgID]
	if !ok {
		notebooks = []prom.Notebook{}
	}
	notebooks = append(notebooks, notebook)
	d.notebooks[notebook.OrgID] = notebooks
	return nil
}

// GetAllNotebooks returns all notebooks for the instance
func (d DB) GetAllNotebooks(orgID string) ([]prom.Notebook, error) {
	notebooks, ok := d.notebooks[orgID]
	if !ok {
		return []prom.Notebook{}, nil
	}
	return notebooks, nil
}

// Close finishes using the db. Noop.
func (d *DB) Close() error {
	return nil
}
