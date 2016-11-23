package db

import (
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/weaveworks/service/configs"
)

// traced adds logrus trace lines on each db call
type traced struct {
	d DB
}

func (t traced) trace(name string, args ...interface{}) {
	logrus.Debugf("%s: %#v", name, args)
}

func (t traced) GetUserConfig(userID configs.UserID, subsystem configs.Subsystem) (cfg configs.Config, err error) {
	defer func() { t.trace("GetUserConfig", userID, subsystem, cfg, err) }()
	return t.d.GetUserConfig(userID, subsystem)
}

func (t traced) SetUserConfig(userID configs.UserID, subsystem configs.Subsystem, cfg configs.Config) (err error) {
	defer func() { t.trace("SetUserConfig", userID, subsystem, cfg, err) }()
	return t.d.SetUserConfig(userID, subsystem, cfg)
}

func (t traced) GetOrgConfig(orgID configs.OrgID, subsystem configs.Subsystem) (cfg configs.Config, err error) {
	defer func() { t.trace("GetOrgConfig", orgID, subsystem, cfg, err) }()
	return t.d.GetOrgConfig(orgID, subsystem)
}

func (t traced) SetOrgConfig(orgID configs.OrgID, subsystem configs.Subsystem, cfg configs.Config) (err error) {
	defer func() { t.trace("SetOrgConfig", orgID, subsystem, cfg, err) }()
	return t.d.SetOrgConfig(orgID, subsystem, cfg)
}

func (t traced) GetCortexConfigs(since time.Duration) (cfgs []*configs.CortexConfig, err error) {
	defer func() { t.trace("GetCortexConfigs", since, cfgs, err) }()
	return t.d.GetCortexConfigs(since)
}

func (t traced) TouchCortexConfig(orgID configs.OrgID) (err error) {
	defer func() { t.trace("TouchCortexConfig", orgID) }()
	return t.d.TouchCortexConfig(orgID)
}

func (t traced) Close() (err error) {
	defer func() { t.trace("Close", err) }()
	return t.d.Close()
}
