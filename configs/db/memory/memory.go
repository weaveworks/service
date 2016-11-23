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
func (d *DB) GetCortexConfigs(since time.Duration) ([]*configs.CortexConfig, error) {
	cfgs := []*configs.CortexConfig{}
	threshold := time.Now().Add(-since)
	for org, subsystems := range d.orgCfgs {
		cfg, ok := subsystems["cortex"]
		if !ok {
			continue
		}
		cortex, err := cfg.ToCortexConfig()
		if err != nil {
			return nil, err
		}
		if cortex.LastEvaluated.After(threshold) {
			continue
		}
		cortex.OrgID = org
		cfgs = append(cfgs, cortex)
	}
	return cfgs, nil
}

// TouchCortexConfig sets the last evaluated time to now.
func (d *DB) TouchCortexConfig(orgID configs.OrgID) error {
	now := time.Now()
	c, ok := d.orgCfgs[orgID]["cortex"]
	if !ok {
		return sql.ErrNoRows
	}
	c["last_evaluated"] = now
	return nil
}

// Close finishes using the db. Noop.
func (d *DB) Close() error {
	return nil
}
