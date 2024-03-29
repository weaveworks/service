package memory

import (
	"errors"
	"sort"
	"time"

	uuid "github.com/satori/go.uuid"
	"github.com/weaveworks/service/notebooks"
)

var currentID = 1

// DB is an in-memory database for testing, and local development
type DB struct {
	notebooks map[string]notebooks.Notebook
	id        uint
}

// New creates a new in-memory database
func New(_, _ string) (*DB, error) {
	return &DB{
		notebooks: map[string]notebooks.Notebook{},
		id:        0,
	}, nil
}

// ByTitle allow you to sort notebooks by name (for test purposes)
type ByTitle []notebooks.Notebook

func (ns ByTitle) Len() int           { return len(ns) }
func (ns ByTitle) Swap(i, j int)      { ns[i], ns[j] = ns[j], ns[i] }
func (ns ByTitle) Less(i, j int) bool { return ns[i].Title < ns[j].Title }

// ListNotebooks returns all notebooks for the instance
func (d DB) ListNotebooks(orgID string) ([]notebooks.Notebook, error) {
	ns := []notebooks.Notebook{}
	for _, notebook := range d.notebooks {
		if notebook.OrgID == orgID {
			ns = append(ns, notebook)
		}
	}
	sort.Sort(ByTitle(ns))
	return ns, nil
}

// CreateNotebook creates a notebook
func (d DB) CreateNotebook(notebook notebooks.Notebook) (string, error) {
	notebook.ID = uuid.NewV4()
	notebook.CreatedAt = time.Now()
	notebook.UpdatedAt = time.Now()
	notebook.Version = uuid.NewV4()

	d.notebooks[notebook.ID.String()] = notebook
	return notebook.ID.String(), nil
}

// GetNotebook returns all notebooks for the instance
func (d DB) GetNotebook(ID, orgID string) (notebooks.Notebook, error) {
	if notebook, ok := d.notebooks[ID]; ok {
		if notebook.OrgID != orgID {
			return notebooks.Notebook{}, errors.New("Notebook not found")
		}
		return notebook, nil
	}
	return notebooks.Notebook{}, errors.New("Notebook not found")
}

// UpdateNotebook updates a notebook
func (d DB) UpdateNotebook(ID, orgID string, update notebooks.Notebook, version string) error {
	if notebook, ok := d.notebooks[ID]; ok {
		if notebook.OrgID != orgID {
			return errors.New("Notebook not found")
		}

		if version != notebook.Version.String() {
			return notebooks.ErrNotebookVersionMismatch
		}

		notebook.UpdatedBy = update.UpdatedBy
		notebook.UpdatedAt = time.Now()
		notebook.Version = uuid.NewV4()
		notebook.Title = update.Title
		notebook.Entries = update.Entries
		notebook.QueryEnd = update.QueryEnd
		notebook.QueryRange = update.QueryRange
		notebook.TrailingNow = update.TrailingNow

		d.notebooks[ID] = notebook
		return nil
	}
	return errors.New("Notebook not found")
}

// DeleteNotebook deletes a notebook
func (d DB) DeleteNotebook(ID, orgID string) error {
	if notebook, ok := d.notebooks[ID]; ok {
		if notebook.OrgID != orgID {
			return errors.New("Notebook not found")
		}
		delete(d.notebooks, ID)
		return nil
	}
	return errors.New("Notebook not found")
}

// Close the database. Noop.
func (d *DB) Close() error {
	return nil
}
