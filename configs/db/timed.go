package db

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/weaveworks/scope/common/instrument"
	"github.com/weaveworks/service/configs"
)

// timed adds prometheus timings to another database implementation
type timed struct {
	d        DB
	Duration *prometheus.HistogramVec
}

func (t timed) errorCode(err error) string {
	switch err {
	case nil:
		return "200"
	default:
		return "500"
	}
}

func (t timed) timeRequest(method string, f func() error) error {
	return instrument.TimeRequestHistogramStatus(method, t.Duration, t.errorCode, f)
}

func (t timed) GetUserConfig(userID configs.UserID, subsystem configs.Subsystem) (cfg configs.Config, err error) {
	t.timeRequest("GetUserConfig", func() error {
		cfg, err = t.d.GetUserConfig(userID, subsystem)
		return err
	})
	return
}

func (t timed) SetUserConfig(userID configs.UserID, subsystem configs.Subsystem, cfg configs.Config) (err error) {
	return t.timeRequest("SetUserConfig", func() error {
		return t.d.SetUserConfig(userID, subsystem, cfg)
	})
}

func (t timed) GetOrgConfig(orgID configs.OrgID, subsystem configs.Subsystem) (cfg configs.Config, err error) {
	t.timeRequest("GetOrgConfig", func() error {
		cfg, err = t.d.GetOrgConfig(orgID, subsystem)
		return err
	})
	return
}

func (t timed) SetOrgConfig(orgID configs.OrgID, subsystem configs.Subsystem, cfg configs.Config) (err error) {
	return t.timeRequest("SetOrgConfig", func() error {
		return t.d.SetOrgConfig(orgID, subsystem, cfg)
	})
}

func (t timed) GetAllOrgConfigs(subsystem configs.Subsystem) (cfgs []*configs.Config, err error) {
	t.timeRequest("GetAllOrgConfigs", func() error {
		cfgs, err = t.d.GetAllOrgConfigs(subsystem)
		return err
	})
	return
}

func (t timed) GetOrgConfigs(subsystem configs.Subsystem, since time.Duration) (cfgs []*configs.Config, err error) {
	t.timeRequest("GetOrgConfigs", func() error {
		cfgs, err = t.d.GetOrgConfigs(subsystem, since)
		return err
	})
	return
}

func (t timed) GetAllUserConfigs(subsystem configs.Subsystem) (cfgs []*configs.Config, err error) {
	t.timeRequest("GetAllUserConfigs", func() error {
		cfgs, err = t.d.GetAllUserConfigs(subsystem)
		return err
	})
	return
}

func (t timed) GetUserConfigs(subsystem configs.Subsystem, since time.Duration) (cfgs []*configs.Config, err error) {
	t.timeRequest("GetUserConfigs", func() error {
		cfgs, err = t.d.GetUserConfigs(subsystem, since)
		return err
	})
	return
}

func (t timed) Close() error {
	return t.timeRequest("Close", func() error {
		return t.d.Close()
	})
}
