package webhook

// EventHandler is the abstraction responsible to handle incoming webhook events.
// It encapsulates logic to:
// - deserialise the event (and ensure it is well-formed in the process),
// - validate that the event is semantically correct,
// - actually process the event.
type EventHandler interface {
	Handle(data []byte) error
}
