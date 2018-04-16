package instance

import (
	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"

	"github.com/weaveworks/service/flux-api/bus"
	"github.com/weaveworks/service/flux-api/history"
	"github.com/weaveworks/service/flux-api/service"
)

// MultitenantInstancer is an implementation of Instancer that communicates with
// Instances over a message bus, in order to support multiple replicas.
type MultitenantInstancer struct {
	DB        ConnectionDB
	Connecter bus.Connecter
	Logger    log.Logger
	History   history.DB
}

// Get gets an Instance by instanceID.
func (m *MultitenantInstancer) Get(instanceID service.InstanceID) (*Instance, error) {
	// Platform interface for this instance
	platform, err := m.Connecter.Connect(instanceID)
	if err != nil {
		return nil, errors.Wrap(err, "connecting to platform")
	}

	// Logger specialised to this instance
	instanceLogger := log.With(m.Logger, "instanceID", instanceID)

	// Events for this instance
	eventRW := eventReadWriter{instanceID, m.History}

	// Configuration for this instance
	config := configurer{instanceID, m.DB}

	return New(
		platform,
		config,
		instanceLogger,
		eventRW,
		eventRW,
	), nil
}
