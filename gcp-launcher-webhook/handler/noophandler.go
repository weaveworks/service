package handler

import (
	log "github.com/sirupsen/logrus"
)

// NoOpHandler is the default handler for Google Cloud Launcher events.
type NoOpHandler struct{}

// Handle does nothing. It does not return any error, so that the event is ACK-ed.
func (h NoOpHandler) Handle(ent *Entitlement) error {
	log.Infof("Nothing to do for entitlement: %v", *ent)
	return nil
}
