package handler

import (
	"encoding/json"

	log "github.com/sirupsen/logrus"
)

// EntitlementHandler i
type EntitlementHandler interface {
	Handle(*Entitlement) error
}

// EventHandler is a composite handler, wrapping around all supported EntitlementHandlers.
// It deserialises the provided event and dispatches it to the appropriate handler.
type EventHandler struct {
	ActivationRequestedHandler EntitlementHandler
	PlanChangeHandler          EntitlementHandler
	CancelledHandler           EntitlementHandler
	SuspendedHandler           EntitlementHandler
	ReactivatedHandler         EntitlementHandler
	DeletedHandler             EntitlementHandler
	DefaultHandler             EntitlementHandler
}

// Handle dispatches the provided event to
func (h EventHandler) Handle(data []byte) error {
	event := Event{}
	if err := json.Unmarshal(data, &event); err != nil {
		log.Error("Failed to deserialise event: ", err)
		return err
	}
	switch event.Type {
	case Cancelled:
		return h.CancelledHandler.Handle(&event.Entitlement)
	case Suspended:
		return h.SuspendedHandler.Handle(&event.Entitlement)
	case Reactivated:
		return h.ReactivatedHandler.Handle(&event.Entitlement)
	case Deleted:
		return h.DeletedHandler.Handle(&event.Entitlement)
	default:
		return h.DefaultHandler.Handle(&event.Entitlement)
	}
}

// EventType represents the various types of events sent by Cloud Launcher.
type EventType string

const (
	// ActivationRequested event.
	ActivationRequested EventType = "ENTITLEMENT_ACTIVATION_REQUESTED"
	// PlanChange event.
	PlanChange EventType = "ENTITLEMENT_PLAN_CHANGE"
	// Cancelled event.
	Cancelled EventType = "ENTITLEMENT_CANCELLED"
	// Suspended event.
	Suspended EventType = "ENTITLEMENT_SUSPENDED"
	// Reactivated event.
	Reactivated EventType = "ENTITLEMENT_REACTIVATED"
	// Deleted event.
	Deleted EventType = "ENTITLEMENT_DELETED"
)

// Event is the JSON format used by Cloud Launcher for the events it sends us.
type Event struct {
	ID          string      `json:"eventId"`
	Type        EventType   `json:"eventType"`
	Entitlement Entitlement `json:"entitlement"`
}

// Entitlement is the JSON format used by Cloud Launcher to model entitlements.
type Entitlement struct {
	ID               string            `json:"id"`
	UpdateTime       string            `json:"updateTime"`
	Account          string            `json:"account,omitempty"`
	Product          string            `json:"product,omitempty"`
	UpcomingPlan     string            `json:"upcoming_plan,omitempty"`
	PlanChangeDate   string            `json:"planChangeDate,omitempty"`
	CancellationDate string            `json:"cancellationDate”,omitempty"`
	Plan             string            `json:"plan,omitempty"`
	InputProperties  map[string]string `json:"inputProperties,omitempty"`
}
