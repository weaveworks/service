package webhook

import (
	"encoding/json"
	"net/http"

	log "github.com/sirupsen/logrus"

	"github.com/weaveworks/service/common/gcp/pubsub/dto"
)

// New returns a http.Handler configured to be able to handle Google Pub/Sub events.
// It requires a MessageHandler to be provided to act upon the message.
func New(handler MessageHandler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		event := dto.Event{}
		if err := json.NewDecoder(req.Body).Decode(&event); err != nil {
			writeError(w, http.StatusBadRequest, req, err) // NACK: we might want to retry on this message later.
			return
		}

		if err := handler.Handle(event.Message); err != nil {
			writeError(w, http.StatusInternalServerError, req, err) // NACK: we might want to retry on this message later.
		} else {
			write(w, http.StatusNoContent) // ACK: remove this message from Pub/Sub.
		}
	})
}

func writeError(w http.ResponseWriter, statusCode int, req *http.Request, err error) {
	write(w, statusCode)
	w.Write([]byte(err.Error()))
	log.Errorf("Failed to process: %v. Error: %v", req, err)
}

func write(w http.ResponseWriter, statusCode int) {
	w.WriteHeader(statusCode)
}
