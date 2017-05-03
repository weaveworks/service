package db

import (
	"flag"
	"fmt"
	"net/url"

	"github.com/weaveworks/service/prom"
	"github.com/weaveworks/service/prom/db/memory"
	"github.com/weaveworks/service/prom/db/postgres"
)

// Config configures the database.
type Config struct {
	URI           string
	MigrationsDir string
}

// RegisterFlags adds the flags required to configure this to the given FlagSet.
func (cfg *Config) RegisterFlags(f *flag.FlagSet) {
	flag.StringVar(&cfg.URI, "database.uri", "postgres://postgres@configs-db.default.svc.cluster.local/prom?sslmode=disable", "URI where the database can be found (for dev you can use memory://)")
	flag.StringVar(&cfg.MigrationsDir, "database.migrations", "", "Path where the database migration files can be found")
}

// DB is the interface for the database.
type DB interface {
	ListNotebooks(orgID string) ([]prom.Notebook, error)
	CreateNotebook(notebook prom.Notebook) error

	GetNotebook(ID, orgID string) (prom.Notebook, error)
	UpdateNotebook(ID, orgID string, notebook prom.Notebook) error
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
