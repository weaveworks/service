package memory

import (
	"errors"

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

// ListNotebooks returns all notebooks for the instance
func (d DB) ListNotebooks(orgID string) ([]prom.Notebook, error) {
	notebooks, ok := d.notebooks[orgID]
	if !ok {
		return []prom.Notebook{}, nil
	}
	return notebooks, nil
}

// CreateNotebook creates a notebook
func (d DB) CreateNotebook(notebook prom.Notebook) error {
	notebooks, ok := d.notebooks[notebook.OrgID]
	if !ok {
		notebooks = []prom.Notebook{}
	}
	notebooks = append(notebooks, notebook)
	d.notebooks[notebook.OrgID] = notebooks
	return nil
}

// GetNotebook returns all notebooks for the instance
func (d DB) GetNotebook(ID, orgID string) (prom.Notebook, error) {
	notebooks, ok := d.notebooks[orgID]
	if !ok {
		return prom.Notebook{}, errors.New("Org not found")
	}

	for _, notebook := range notebooks {
		if notebook.ID.String() == ID {
			return notebook, nil
		}
	}
	return prom.Notebook{}, errors.New("Notebook not found")
}

// UpdateNotebook updates a notebook
func (d DB) UpdateNotebook(ID, orgID string, update prom.Notebook) error {
	notebooks, ok := d.notebooks[orgID]
	if !ok {
		return errors.New("Org not found")
	}

	var updatedNotebooks []prom.Notebook
	for _, notebook := range notebooks {
		if notebook.ID.String() == ID {
			notebook.Title = update.Title
			notebook.AuthorID = update.AuthorID
			notebook.UpdatedAt = update.UpdatedAt
			notebook.Entries = update.Entries
		}
		updatedNotebooks = append(updatedNotebooks, notebook)
	}
	d.notebooks[orgID] = updatedNotebooks
	return nil
}

// DeleteNotebook deletes a notebook
func (d DB) DeleteNotebook(ID, orgID string) error {
	notebooks, ok := d.notebooks[orgID]
	if !ok {
		return errors.New("Org not found")
	}

	var updatedNotebooks []prom.Notebook
	for _, notebook := range notebooks {
		if notebook.ID.String() != ID {
			updatedNotebooks = append(updatedNotebooks, notebook)
		}
	}
	d.notebooks[orgID] = updatedNotebooks
	return nil
}

// Close finishes using the db. Noop.
func (d *DB) Close() error {
	return nil
}
