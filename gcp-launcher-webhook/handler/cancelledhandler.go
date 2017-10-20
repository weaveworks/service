package handler

// CancelledHandler handles Google Cloud Launcher events representing the cancellation of a subscription.
type CancelledHandler struct{}

// Handle TODO.
func (h CancelledHandler) Handle(ent *Entitlement) error {
	return nil
}
