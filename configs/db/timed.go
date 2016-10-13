package db

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/weaveworks/scope/common/instrument"
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

func (t timed) Close() error {
	return t.timeRequest("Close", func() error {
		return t.d.Close()
	})
}
