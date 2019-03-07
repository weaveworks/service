package event

// Type represents a message event type.
type Type string

// Event types for entitlement messages which somewhat map
// to their procurement.EntitlementState counterpart.
const (
	CreationRequested    Type = "ENTITLEMENT_CREATION_REQUESTED"
	Active               Type = "ENTITLEMENT_ACTIVE"
	PlanChangeRequested  Type = "ENTITLEMENT_PLAN_CHANGE_REQUESTED"
	PlanChanged          Type = "ENTITLEMENT_PLAN_CHANGED"
	PlanChangeCancelled  Type = "ENTITLEMENT_PLAN_CHANGE_CANCELLED"
	Cancelled            Type = "ENTITLEMENT_CANCELLED"
	PendingCancellation  Type = "ENTITLEMENT_PENDING_CANCELLATION"
	CancellationReverted Type = "ENTITLEMENT_CANCELLATION_REVERTED"
	Deleted              Type = "ENTITLEMENT_DELETED"
)

// Payload is passed as data of a dto.Message.
type Payload struct {
	EventID   string `json:"eventId"`
	EventType Type   `json:"eventType"`

	Entitlement struct {
		ID      string `json:"id"`
		NewPlan string `json:"newPlan"`
	} `json:"entitlement"`

	Account struct {
		ID string `json:"id"`
	} `json:"account"`
}
