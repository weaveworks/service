package db

import (
	"net/url"
	"time"

	"github.com/Sirupsen/logrus"

	"github.com/weaveworks/service/configs"
	"github.com/weaveworks/service/configs/db/memory"
	"github.com/weaveworks/service/configs/db/postgres"
	"github.com/weaveworks/service/users" // For instrumentation.
)

// DB is the interface for the database.
type DB interface {
	GetUserConfig(userID configs.UserID, subsystem configs.Subsystem) (configs.ConfigView, error)
	SetUserConfig(userID configs.UserID, subsystem configs.Subsystem, cfg configs.Config) error
	GetOrgConfig(orgID configs.OrgID, subsystem configs.Subsystem) (configs.ConfigView, error)
	SetOrgConfig(orgID configs.OrgID, subsystem configs.Subsystem, cfg configs.Config) error

	GetAllOrgConfigs(subsystem configs.Subsystem) (map[configs.OrgID]configs.Config, error)
	GetOrgConfigs(subsystem configs.Subsystem, since time.Duration) (map[configs.OrgID]configs.Config, error)
	GetAllUserConfigs(subsystem configs.Subsystem) (map[configs.UserID]configs.Config, error)
	GetUserConfigs(subsystem configs.Subsystem, since time.Duration) (map[configs.UserID]configs.Config, error)

	Close() error
}

// MustNew creates a new database from the URI, or panics.
// XXX: Copied from `users/db/db.go`.
func MustNew(databaseURI, migrationsDir string) DB {
	u, err := url.Parse(databaseURI)
	if err != nil {
		logrus.Fatal(err)
	}
	var d DB
	switch u.Scheme {
	case "memory":
		d, err = memory.New(databaseURI, migrationsDir)
	case "postgres":
		d, err = postgres.New(databaseURI, migrationsDir)
	default:
		logrus.Fatalf("Unknown database type: %s", u.Scheme)
	}
	if err != nil {
		logrus.Fatal(err)
	}
	// XXX: Current instrumentation doesn't provide a way to distinguish
	// between backend databases (e.g. configs, users).
	return traced{timed{d, users.DatabaseRequestDuration}}
}
