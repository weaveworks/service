package db

import (
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

func (t timed) SetUserConfig(userID configs.UserID, subsystem configs.Subsystem, cfg configs.Config) (created bool, err error) {
	t.timeRequest("SetUserConfig", func() error {
		created, err = t.d.SetUserConfig(userID, subsystem, cfg)
		return err
	})
	return
}

func (t timed) Close() error {
	return t.timeRequest("Close", func() error {
		return t.d.Close()
	})
}
