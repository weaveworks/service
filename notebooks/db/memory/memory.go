package memory

import (
	"errors"
	"time"

	"github.com/weaveworks/service/notebooks"
)

var currentID = 1

// DB is an in-memory database for testing, and local development
type DB struct {
	notebooks map[string][]notebooks.Notebook
	id        uint
}

// New creates a new in-memory database
func New(_, _ string) (*DB, error) {
	return &DB{
		notebooks: map[string][]notebooks.Notebook{},
		id:        0,
	}, nil
}

// ListNotebooks returns all notebooks for the instance
func (d DB) ListNotebooks(orgID string) ([]notebooks.Notebook, error) {
	ns, ok := d.notebooks[orgID]
	if !ok {
		return []notebooks.Notebook{}, nil
	}
	return ns, nil
}

// CreateNotebook creates a notebook
func (d DB) CreateNotebook(notebook notebooks.Notebook) error {
	ns, ok := d.notebooks[notebook.OrgID]
	if !ok {
		ns = []notebooks.Notebook{}
	}
	ns = append(ns, notebook)
	d.notebooks[notebook.OrgID] = ns
	return nil
}

// GetNotebook returns all notebooks for the instance
func (d DB) GetNotebook(ID, orgID string) (notebooks.Notebook, error) {
	ns, ok := d.notebooks[orgID]
	if !ok {
		return notebooks.Notebook{}, errors.New("Org not found")
	}

	for _, notebook := range ns {
		if notebook.ID.String() == ID {
			return notebook, nil
		}
	}
	return notebooks.Notebook{}, errors.New("Notebook not found")
}

// UpdateNotebook updates a notebook
func (d DB) UpdateNotebook(ID, orgID string, update notebooks.Notebook) error {
	ns, ok := d.notebooks[orgID]
	if !ok {
		return errors.New("Org not found")
	}

	var updatedNotebooks []notebooks.Notebook
	for _, notebook := range ns {
		if notebook.ID.String() == ID {
			notebook.UpdatedBy = update.UpdatedBy
			notebook.UpdatedAt = time.Now()
			notebook.Title = update.Title
			notebook.Entries = update.Entries
		}
		updatedNotebooks = append(updatedNotebooks, notebook)
	}
	d.notebooks[orgID] = updatedNotebooks
	return nil
}

// DeleteNotebook deletes a notebook
func (d DB) DeleteNotebook(ID, orgID string) error {
	ns, ok := d.notebooks[orgID]
	if !ok {
		return errors.New("Org not found")
	}

	var updatedNotebooks []notebooks.Notebook
	for _, notebook := range ns {
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
