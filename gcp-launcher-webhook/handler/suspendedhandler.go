package handler

// SuspendedHandler handles Google Cloud Launcher events representing the suspension of a subscription.
type SuspendedHandler struct{}

// Handle TODO.
func (h SuspendedHandler) Handle(ent *Entitlement) error {
	return nil
}
