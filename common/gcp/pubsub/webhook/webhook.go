package webhook

import (
	"io/ioutil"
	"net/http"

	log "github.com/sirupsen/logrus"

	"github.com/weaveworks/service/common/gcp/pubsub/dto"
)

// New returns a http.Handler configured to be able to handle Google Pub/Sub events.
// It requires a EventHandler to be provided to act upon the event.
func New(handler EventHandler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			writeError(w, http.StatusInternalServerError, req, err) // NACK: we might want to retry on this message later.
			return
		}
		event := dto.Event{}
		if err := event.Unmarshal(body); err != nil {
			writeError(w, http.StatusBadRequest, req, err) // NACK: we might want to retry on this message later.
			return
		}
		if err := event.Message.Decode(); err != nil {
			writeError(w, http.StatusBadRequest, req, err) // NACK: we might want to retry on this message later.
			return
		}
		if err := handler.Handle(event.Message.Bytes); err != nil {
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
