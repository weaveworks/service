package eventmanager

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

// Register HTTP handlers
func (em *EventManager) Register(r *mux.Router) {
	for _, route := range []struct {
		name, method, path string
		handler            http.Handler
	}{
		{"list_event_types", "GET", "/api/notification/config/eventtypes", withNoArgs(em.handleGetEventTypes)},
		{"list_receivers", "GET", "/api/notification/config/receivers", withInstance(em.handleListReceivers)},
		{"create_receiver", "POST", "/api/notification/config/receivers", withInstance(em.handleCreateReceiver)},
		{"get_receiver", "GET", "/api/notification/config/receivers/{id}", withInstanceAndID(em.handleGetReceiver)},
		{"update_receiver", "PUT", "/api/notification/config/receivers/{id}", withInstanceAndID(em.handleUpdateReceiver)},
		{"delete_receiver", "DELETE", "/api/notification/config/receivers/{id}", withInstanceAndID(em.handleDeleteReceiver)},
		{"get_events", "GET", "/api/notification/events", withInstance(em.handleGetEvents)},

		// -- Internal API
		{"create_test_event", "POST", "/api/notification/testevent", em.rateLimited(em.handleTestEvent)},
		{"create_event", "POST", "/api/notification/events", em.rateLimited(em.handleCreateEvent)},
		// Legacy event handler
		{"create_slack_event", "POST", "/api/notification/slack/{instanceID}/{eventType}", em.rateLimited(em.handleSlackEvent)},
		{"health_check", "GET", "/api/notification/events/healthcheck", http.HandlerFunc(em.handleHealthCheck)},

		// -- External API - reachable from outside Weave Cloud cluster
		{"create_event_external", "POST", "/api/notification/external/events", em.rateLimited(em.handleCreateEvent)},
	} {
		r.Handle(route.path, route.handler).Methods(route.method).Name(route.name)
	}
}

// rateLimited is rate limit middleware
func (em *EventManager) rateLimited(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !em.limiter.Allow() {
			log.Warnf("too many %s requests to %s, request is not allowed", r.Method, r.URL.Path)
			rateLimitedRequests.With(prometheus.Labels{"method": r.Method, "path": r.URL.Path}).Inc()
			w.WriteHeader(http.StatusTooManyRequests)
			requestsError.With(prometheus.Labels{"status_code": http.StatusText(http.StatusTooManyRequests)}).Inc()
			return
		}
		next(w, r)
	}
}
