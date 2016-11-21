package memory

import (
	"database/sql"
	"time"

	"github.com/weaveworks/service/configs"
)

// DB is an in-memory database for testing, and local development
type DB struct {
	userCfgs map[configs.UserID]map[configs.Subsystem]configs.Config
	orgCfgs  map[configs.OrgID]map[configs.Subsystem]configs.Config
}

// New creates a new in-memory database
func New(_, _ string) (*DB, error) {
	return &DB{
		userCfgs: map[configs.UserID]map[configs.Subsystem]configs.Config{},
		orgCfgs:  map[configs.OrgID]map[configs.Subsystem]configs.Config{},
	}, nil
}

// GetUserConfig gets the user's configuration.
func (d *DB) GetUserConfig(userID configs.UserID, subsystem configs.Subsystem) (configs.Config, error) {
	cfg, ok := d.userCfgs[userID][subsystem]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return cfg, nil
}

// SetUserConfig sets configuration for a user.
func (d *DB) SetUserConfig(userID configs.UserID, subsystem configs.Subsystem, cfg configs.Config) error {
	// XXX: Is this really how you assign a thing to a nested map?
	user, ok := d.userCfgs[userID]
	if !ok {
		user = map[configs.Subsystem]configs.Config{}
	}
	user[subsystem] = cfg
	d.userCfgs[userID] = user
	return nil
}

// GetOrgConfig gets the org's configuration.
func (d *DB) GetOrgConfig(orgID configs.OrgID, subsystem configs.Subsystem) (configs.Config, error) {
	cfg, ok := d.orgCfgs[orgID][subsystem]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return cfg, nil
}

// SetOrgConfig sets configuration for a org.
func (d *DB) SetOrgConfig(orgID configs.OrgID, subsystem configs.Subsystem, cfg configs.Config) error {
	// XXX: Is this really how you assign a thing to a nested map?
	org, ok := d.orgCfgs[orgID]
	if !ok {
		org = map[configs.Subsystem]configs.Config{}
	}
	org[subsystem] = cfg
	d.orgCfgs[orgID] = org
	return nil
}

// GetCortexConfigs returns all the configurations for cortex that haven't
// been evaluated since the given time.
func (d *DB) GetCortexConfigs(since time.Duration) ([]configs.CortexConfig, error) {
	cfgs := []configs.CortexConfig{}
	for org, subsystems := range d.orgCfgs {
		cortex := subsystems["cortex"]
		for _, cfg := range cortex {
			cortexCfg := cfg.(configs.CortexConfig)
			cortexCfg.OrgID = org
			cfgs = append(cfgs, cortexCfg)
		}
	}
	return cfgs, nil
}

// Close finishes using the db. Noop.
func (d *DB) Close() error {
	return nil
}
