package history

import (
	"fmt"
	"time"

	"github.com/go-kit/kit/metrics/prometheus"
	stdprometheus "github.com/prometheus/client_golang/prometheus"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/event"
	"github.com/weaveworks/service/flux-api/service"
)

const (
	labelMethod  = "method"
	labelSuccess = "success"
)

var (
	requestDuration = prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
		Namespace: "flux",
		Subsystem: "history",
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

func (i *instrumentedDB) LogEvent(inst service.InstanceID, e event.Event) (err error) {
	defer func(begin time.Time) {
		requestDuration.With(
			labelMethod, "LogEvent",
			labelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.db.LogEvent(inst, e)
}

func (i *instrumentedDB) AllEvents(inst service.InstanceID, before time.Time, limit int64, after time.Time) (e []event.Event, err error) {
	defer func(begin time.Time) {
		requestDuration.With(
			labelMethod, "AllEvents",
			labelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.db.AllEvents(inst, before, limit, after)
}

func (i *instrumentedDB) EventsForService(inst service.InstanceID, s flux.ResourceID, before time.Time, limit int64, after time.Time) (e []event.Event, err error) {
	defer func(begin time.Time) {
		requestDuration.With(
			labelMethod, "EventsForService",
			labelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.db.EventsForService(inst, s, before, limit, after)
}

func (i *instrumentedDB) GetEvent(id event.EventID) (e event.Event, err error) {
	defer func(begin time.Time) {
		requestDuration.With(
			labelMethod, "GetEvent",
			labelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.db.GetEvent(id)
}

func (i *instrumentedDB) Close() (err error) {
	defer func(begin time.Time) {
		requestDuration.With(
			labelMethod, "Close",
			labelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.db.Close()
}
