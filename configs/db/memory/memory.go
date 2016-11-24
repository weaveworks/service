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

// GetAllOrgConfigs gets all of the organization configs for a subsystem.
func (d *DB) GetAllOrgConfigs(subsystem configs.Subsystem) (map[configs.OrgID]configs.Config, error) {
	cfgs := map[configs.OrgID]configs.Config{}
	for org, subsystems := range d.orgCfgs {
		cfg, ok := subsystems[subsystem]
		if ok {
			cfgs[org] = cfg
		}
	}
	return cfgs, nil
}

// GetOrgConfigs gets all of the organization configs for a subsystem that
// have changed recently.
func (d *DB) GetOrgConfigs(subsystem configs.Subsystem, since time.Duration) (map[configs.OrgID]configs.Config, error) {
	return map[configs.OrgID]configs.Config{}, nil
}

// GetAllUserConfigs gets all of the user configs for a subsystem.
func (d *DB) GetAllUserConfigs(subsystem configs.Subsystem) (map[configs.UserID]configs.Config, error) {
	cfgs := map[configs.UserID]configs.Config{}
	for user, subsystems := range d.userCfgs {
		cfg, ok := subsystems[subsystem]
		if ok {
			cfgs[user] = cfg
		}
	}
	return cfgs, nil
}

// GetUserConfigs gets all of the user configs for a subsystem that have
// changed recently.
func (d *DB) GetUserConfigs(subsystem configs.Subsystem, since time.Duration) (map[configs.UserID]configs.Config, error) {
	return map[configs.UserID]configs.Config{}, nil
}

// Close finishes using the db. Noop.
func (d *DB) Close() error {
	return nil
}
