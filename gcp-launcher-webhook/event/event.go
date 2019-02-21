package event

type EventType string

const (
	CreationRequested    EventType = "ENTITLEMENT_CREATION_REQUESTED"
	Active               EventType = "ENTITLEMENT_ACTIVE"
	PlanChangeRequested  EventType = "ENTITLEMENT_PLAN_CHANGE_REQUESTED"
	PlanChanged          EventType = "ENTITLEMENT_PLAN_CHANGED"
	PlanChangeCancelled  EventType = "ENTITLEMENT_PLAN_CHANGE_CANCELLED"
	Cancelled            EventType = "ENTITLEMENT_CANCELLED"
	PendingCancellation  EventType = "ENTITLEMENT_PENDING_CANCELLATION"
	CancellationReverted EventType = "ENTITLEMENT_CANCELLATION_REVERTED"
	Deleted              EventType = "ENTITLEMENT_DELETED"
)

type Payload struct {
	EventID   string    `json:"eventId"`
	EventType EventType `json:"eventType"`

	Entitlement struct {
		ID      string `json:"id"`
		NewPlan string `json:"newPlan"`
	} `json:"entitlement"`

	Account struct {
		ID string `json:"id"`
	} `json:"account"`
}
