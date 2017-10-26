package instance

import (
	"github.com/go-kit/kit/log"

	"github.com/weaveworks/flux/event"
	"github.com/weaveworks/flux/remote"
	"github.com/weaveworks/service/flux-api/history"
	"github.com/weaveworks/service/flux-api/service"
)

type Instancer interface {
	Get(inst service.InstanceID) (*Instance, error)
}

type Instance struct {
	Platform remote.Platform
	Config   configurer

	log.Logger
	history.EventReader
	event.EventWriter
}

func New(
	platform remote.Platform,
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
