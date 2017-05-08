package db

import (
	"flag"
	"fmt"
	"net/url"

	"github.com/weaveworks/service/notebooks"
	"github.com/weaveworks/service/notebooks/db/memory"
	"github.com/weaveworks/service/notebooks/db/postgres"
)

// Config configures the database.
type Config struct {
	URI           string
	MigrationsDir string
}

// RegisterFlags adds the flags required to configure this to the given FlagSet.
func (cfg *Config) RegisterFlags(f *flag.FlagSet) {
	flag.StringVar(&cfg.URI, "database.uri", "postgres://postgres@configs-db.default.svc.cluster.local/notebooks?sslmode=disable", "URI where the database can be found (for dev you can use memory://)")
	flag.StringVar(&cfg.MigrationsDir, "database.migrations", "", "Path where the database migration files can be found")
}

// DB is the interface for the database.
type DB interface {
	ListNotebooks(orgID string) ([]notebooks.Notebook, error)
	CreateNotebook(notebook notebooks.Notebook) (string, error)

	GetNotebook(ID, orgID string) (notebooks.Notebook, error)
	UpdateNotebook(ID, orgID string, notebook notebooks.Notebook) error
	DeleteNotebook(ID, orgID string) error

	Close() error
}

// New creates a new database.
func New(cfg Config) (DB, error) {
	u, err := url.Parse(cfg.URI)
	if err != nil {
		return nil, err
	}
	var d DB
	switch u.Scheme {
	case "memory":
		d, err = memory.New(cfg.URI, cfg.MigrationsDir)
	case "postgres":
		d, err = postgres.New(cfg.URI, cfg.MigrationsDir)
	default:
		return nil, fmt.Errorf("Unknown database type: %s", u.Scheme)
	}
	if err != nil {
		return nil, err
	}
	return d, nil
}
