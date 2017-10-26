package webhook

import "github.com/weaveworks/service/common/gcp/pubsub/dto"

// MessageHandler is the abstraction responsible to handle incoming webhook events.
type MessageHandler interface {
	Handle(dto.Message) error
}
