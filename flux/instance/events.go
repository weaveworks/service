package instance

import (
	"time"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/event"
	"github.com/weaveworks/service/flux/history"
	"github.com/weaveworks/service/flux/service"
)

type EventReadWriter struct {
	inst service.InstanceID
	db   history.DB
}

func (rw EventReadWriter) LogEvent(e event.Event) error {
	return rw.db.LogEvent(rw.inst, e)
}

func (rw EventReadWriter) AllEvents(before time.Time, limit int64, after time.Time) ([]event.Event, error) {
	return rw.db.AllEvents(rw.inst, before, limit, after)
}

func (rw EventReadWriter) EventsForService(service flux.ResourceID, before time.Time, limit int64, after time.Time) ([]event.Event, error) {
	return rw.db.EventsForService(rw.inst, service, before, limit, after)
}

func (rw EventReadWriter) GetEvent(id event.EventID) (event.Event, error) {
	return rw.db.GetEvent(id)
}