package webhook

import (
	"encoding/json"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"

	"github.com/weaveworks/service/common/gcp/pubsub/dto"
	"github.com/weaveworks/service/common/render"
	"github.com/weaveworks/service/users"
	users_render "github.com/weaveworks/service/users/render"
)

var (
	receivedMessages = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "service",
		Subsystem: "pubsub",
		Name:      "received_messages_total",
		Help:      "Number of received messages by the PubSub webhook.",
	}, []string{"status"})
)

func init() {
	prometheus.MustRegister(receivedMessages)
}

// New returns a http.Handler configured to be able to handle Google Pub/Sub events.
// It requires a MessageHandler to be provided to act upon the message.
func New(handler MessageHandler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		var err error

		event := dto.Event{}
		if err = json.NewDecoder(req.Body).Decode(&event); err != nil {
			err = users.NewMalformedInputError(err) // NACK: we might want to retry on this message later.
		} else {
			log.Infof("Incoming webhook event: %+v", event)
			err = handler.Handle(event.Message)
		}

		if err != nil {
			render.Error(w, req, err, users_render.ErrorStatusCode) // NACK: we might want to retry on this message later.
			receivedMessages.WithLabelValues("error").Inc()
		} else {
			w.WriteHeader(http.StatusNoContent) // ACK: remove this message from Pub/Sub.
			receivedMessages.WithLabelValues("success").Inc()
		}
	})
}
