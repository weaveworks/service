package history

import (
	"io"
	"time"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/event"

	"github.com/weaveworks/service/flux-api/service"
)

// EventReader is the read-interface for event storage.
type EventReader interface {
	// AllEvents returns a history for every service. Events must be
	// returned in descending timestamp order.
	AllEvents(time.Time, int64, time.Time) ([]event.Event, error)

	// EventsForService returns the history for a particular
	// service. Events must be returned in descending timestamp order.
	EventsForService(flux.ResourceID, time.Time, int64, time.Time) ([]event.Event, error)

	// GetEvent finds a single event, by ID.
	GetEvent(event.EventID) (event.Event, error)
}

// DB is the write-interface for event storage.
type DB interface {
	LogEvent(service.InstanceID, event.Event) error
	AllEvents(service.InstanceID, time.Time, int64, time.Time) ([]event.Event, error)
	EventsForService(service.InstanceID, flux.ResourceID, time.Time, int64, time.Time) ([]event.Event, error)
	GetEvent(event.EventID) (event.Event, error)
	io.Closer
}
