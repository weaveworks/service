package memory

import (
	"database/sql"

	"github.com/weaveworks/service/configs"
)

// DB is an in-memory database for testing, and local development
type DB struct {
	last configs.Config
}

// New creates a new in-memory database
func New(_, _ string) (*DB, error) {
	return &DB{last: nil}, nil
}

// GetUserConfig gets the user's configuration.
func (d *DB) GetUserConfig(userID configs.UserID, subsystem configs.Subsystem) (configs.Config, error) {
	if d.last == nil {
		return nil, sql.ErrNoRows
	}
	return d.last, nil
}

// SetUserConfig sets configuration for a user.
func (d *DB) SetUserConfig(userID configs.UserID, subsystem configs.Subsystem, cfg configs.Config) error {
	d.last = cfg
	return nil
}

// Close finishes using the db. Noop.
func (d *DB) Close() error {
	return nil
}
