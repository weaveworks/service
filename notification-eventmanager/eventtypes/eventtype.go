package eventtypes

type EventType interface {
	Render(recv string) string
}
