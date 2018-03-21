package instance

import (
	"fmt"
	"time"

	"github.com/go-kit/kit/metrics/prometheus"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
	fluxmetrics "github.com/weaveworks/flux/metrics"
	"github.com/weaveworks/service/flux-api/service"
)

const (
	labelMethod  = "method"
	labelSuccess = "success"
)

var (
	releaseHelperDuration = prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
		Namespace: "flux",
		Subsystem: "fluxsvc",
		Name:      "release_helper_duration_seconds",
		Help:      "Duration in seconds of a variety of release helper methods.",
		Buckets:   stdprometheus.DefBuckets,
	}, []string{fluxmetrics.LabelMethod, fluxmetrics.LabelSuccess})
	requestDuration = prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
		Namespace: "flux",
		Subsystem: "instance",
		Name:      "request_duration_seconds",
		Help:      "Request duration in seconds.",
		Buckets:   stdprometheus.DefBuckets,
	}, []string{labelMethod, labelSuccess})
)

type instrumentedDB struct {
	db DB
}

// InstrumentedDB wraps a DB instance in instrumentation.
func InstrumentedDB(db DB) DB {
	return &instrumentedDB{db}
}

func (i *instrumentedDB) UpdateConnection(inst service.InstanceID, update UpdateFunc) (err error) {
	defer func(begin time.Time) {
		requestDuration.With(
			labelMethod, "UpdateConnection",
			labelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.db.UpdateConnection(inst, update)
}

func (i *instrumentedDB) GetConnection(inst service.InstanceID) (c Connection, err error) {
	defer func(begin time.Time) {
		requestDuration.With(
			labelMethod, "GetConnection",
			labelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.db.GetConnection(inst)
}
