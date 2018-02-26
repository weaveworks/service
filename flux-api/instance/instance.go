package instance

import (
	"github.com/go-kit/kit/log"

	"github.com/weaveworks/flux/api"
	"github.com/weaveworks/flux/event"
	"github.com/weaveworks/service/flux-api/history"
	"github.com/weaveworks/service/flux-api/service"
)

// Instancer gets an Instance by instanceID.
type Instancer interface {
	Get(inst service.InstanceID) (*Instance, error)
}

// Instance is a flux-api connected flux daemon instance.
type Instance struct {
	Platform api.UpstreamServer
	Config   configurer

	log.Logger
	history.EventReader
	event.EventWriter
}

// New creates a new Instance.
func New(
	platform api.UpstreamServer,
	config configurer,
	logger log.Logger,
	events history.EventReader,
	eventlog event.EventWriter,
) *Instance {
	return &Instance{
		Platform:    platform,
		Config:      config,
		Logger:      logger,
		EventReader: events,
		EventWriter: eventlog,
	}
}
