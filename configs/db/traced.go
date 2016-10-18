package db

import (
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

func (t traced) Close() (err error) {
	defer func() { t.trace("Close", err) }()
	return t.d.Close()
}
