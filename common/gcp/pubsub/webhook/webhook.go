package webhook

import (
	"encoding/json"
	"net/http"

	log "github.com/sirupsen/logrus"

	"github.com/weaveworks/service/common/gcp/pubsub/dto"
	"github.com/weaveworks/service/users/render"
	"github.com/weaveworks/service/users"
)

// New returns a http.Handler configured to be able to handle Google Pub/Sub events.
// It requires a MessageHandler to be provided to act upon the message.
func New(handler MessageHandler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		event := dto.Event{}
		if err := json.NewDecoder(req.Body).Decode(&event); err != nil {
			render.Error(w, req, users.MalformedInputError(err)) // NACK: we might want to retry on this message later.
			return
		}
		log.Debugf("Incoming webhook event: %+v", event)

		if err := handler.Handle(event.Message); err != nil {
			render.Error(w, req, err) // NACK: we might want to retry on this message later.
		} else {
			w.WriteHeader(http.StatusNoContent) // ACK: remove this message from Pub/Sub.
		}
	})
}
