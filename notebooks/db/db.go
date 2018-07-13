package db

import (
	"fmt"

	"github.com/weaveworks/service/common/dbconfig"
	"github.com/weaveworks/service/notebooks"
	"github.com/weaveworks/service/notebooks/db/memory"
	"github.com/weaveworks/service/notebooks/db/postgres"
)

// DB is the interface for the database.
type DB interface {
	ListNotebooks(orgID string) ([]notebooks.Notebook, error)
	CreateNotebook(notebook notebooks.Notebook) (string, error)

	GetNotebook(ID, orgID string) (notebooks.Notebook, error)
	UpdateNotebook(ID, orgID string, notebook notebooks.Notebook, version string) error
	DeleteNotebook(ID, orgID string) error

	Close() error
}

// New creates a new database.
func New(cfg dbconfig.Config) (DB, error) {
	scheme, dataSourceName, migrationsDir, err := cfg.Parameters()
	if err != nil {
		return nil, err
	}
	var d DB
	switch scheme {
	case "memory":
		d, err = memory.New(dataSourceName, migrationsDir)
	case "postgres":
		d, err = postgres.New(dataSourceName, migrationsDir)
	default:
		return nil, fmt.Errorf("Unknown database type: %s", scheme)
	}
	if err != nil {
		return nil, err
	}
	return d, nil
}
