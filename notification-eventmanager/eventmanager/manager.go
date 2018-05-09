package eventmanager

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"sync"

	"github.com/aws/aws-sdk-go/service/sqs/sqsiface"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
	_ "gopkg.in/mattes/migrate.v1/driver/postgres" // Import the postgres migrations driver

	"github.com/weaveworks/service/notification-eventmanager/db"
	"github.com/weaveworks/service/notification-eventmanager/types"
	"github.com/weaveworks/service/users"
)

const (
	batchSize            = 10
	ratelimit            = 100
	markdownNewline      = "  \n"
	markdownNewParagraph = "\n\n"
)

var (
	requestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "incoming_event_requests_total",
		Help: "Number of incoming event requests.",
	}, []string{"handler"})

	requestsError = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "incoming_event_request_errors_total",
		Help: "Number of errors in incoming event requests.",
	}, []string{"status_code"})

	rateLimitedRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "rate_limited_event_requests_total",
		Help: "Number of incoming event requests not allowed because of rate limit.",
	}, []string{"method", "path"})

	eventsToDBTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "events_to_db_total",
		Help: "Number of events sent to db.",
	}, []string{"event_type"})

	eventsToDBError = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "event_to_db_errors_total",
		Help: "Number of errors sending event to db.",
	}, []string{"event_type"})

	eventsToSQSTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "events_to_sqs_total",
		Help: "Number of events for which notifications were enqueued into SQS.",
	}, []string{"event_type"})

	eventsToSQSError = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "event_to_sqs_errors_total",
		Help: "Number of errors enqueueing notification batches into SQS.",
	}, []string{"event_type"})
)

// EventManager contains Users service client, DB connection and SQS queue to store events into DB and send notifications to SQS queue
type EventManager struct {
	UsersClient users.UsersClient
	DB          db.DB
	SQSClient   sqsiface.SQSAPI
	SQSQueue    string
	wg          sync.WaitGroup
	limiter     *rate.Limiter
}

func init() {
	prometheus.MustRegister(
		requestsTotal,
		requestsError,
		rateLimitedRequests,
		eventsToDBTotal,
		eventsToDBError,
		eventsToSQSTotal,
		eventsToSQSError,
	)
}

// New creates new EventManager
func New(usersClient users.UsersClient, db db.DB, sqsClient sqsiface.SQSAPI, sqsQueue string) *EventManager {
	return &EventManager{
		UsersClient: usersClient,
		DB:          db,
		SQSClient:   sqsClient,
		SQSQueue:    sqsQueue,
		limiter:     rate.NewLimiter(ratelimit, ratelimit),
	}
}

// TestEventHandler creates test event

// HandleHealthCheck handles a very simple health check
func (em *EventManager) HandleHealthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// SlackHandler handles slack json payload that includes the message text and some options, creates event, log it in DB and queue
func (em *EventManager) SlackHandler(w http.ResponseWriter, r *http.Request) {
	requestsTotal.With(prometheus.Labels{"handler": "SlackHandler"}).Inc()
	vars := mux.Vars(r)

	eventType := vars["eventType"]
	if eventType == "" {
		eventsToSQSError.With(prometheus.Labels{"event_type": "empty"}).Inc()
		log.Errorf("eventType is empty in request %s", r.URL)
		http.Error(w, "eventType is empty in request", http.StatusBadRequest)
		requestsError.With(prometheus.Labels{"status_code": http.StatusText(http.StatusBadRequest)}).Inc()
		return
	}
	log.Debugf("eventType = %s", eventType)

	instanceID := vars["instanceID"]
	if instanceID == "" {
		requestsError.With(prometheus.Labels{"status_code": http.StatusText(http.StatusBadRequest)}).Inc()
		log.Errorf("instanceID is empty in request %s", r.URL)
		http.Error(w, "instanceID is empty in request", http.StatusBadRequest)
		return
	}
	log.Debugf("instanceID = %s", instanceID)

	instanceData, err := em.UsersClient.GetOrganization(r.Context(), &users.GetOrganizationRequest{
		ID: &users.GetOrganizationRequest_InternalID{InternalID: instanceID},
	})
	if err != nil {
		if isStatusErrorCode(err, http.StatusNotFound) {
			log.Warnf("instance name for ID %s not found for event type %s", instanceID, eventType)
			http.Error(w, "Instance not found", http.StatusNotFound)
			requestsError.With(prometheus.Labels{"status_code": http.StatusText(http.StatusNotFound)}).Inc()
			return
		}
		log.Errorf("error requesting instance data from users service for event type %s: %s", eventType, err)
		http.Error(w, "unable to retrieve instance data", http.StatusInternalServerError)
		return
	}
	log.Debugf("Got data from users service: %v", instanceData.Organization.Name)

	defer r.Body.Close()
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		requestsError.With(prometheus.Labels{"status_code": http.StatusText(http.StatusBadRequest)}).Inc()
		log.Errorf("cannot read body for request %s", r.URL)
		http.Error(w, "cannot read body", http.StatusBadRequest)
		return
	}

	var sm types.SlackMessage
	if err = json.Unmarshal(body, &sm); err != nil {
		requestsError.With(prometheus.Labels{"status_code": http.StatusText(http.StatusBadRequest)}).Inc()
		log.Errorf("cannot unmarshal body to SlackMessage struct, error: %s", err)
		http.Error(w, "cannot unmarshal body", http.StatusBadRequest)
		return
	}

	e, err := buildEvent(body, sm, eventType, instanceID, instanceData.Organization.Name)
	if err != nil {
		requestsError.With(prometheus.Labels{"status_code": http.StatusText(http.StatusBadRequest)}).Inc()
		log.Errorf("cannot build event, error: %s", err)
		http.Error(w, "cannot build event", http.StatusBadRequest)
		return
	}

	if err := em.storeAndSend(r.Context(), e, instanceData.Organization.FeatureFlags); err != nil {
		log.Errorf("cannot post and send %s event, error: %s", e.Type, err)
		http.Error(w, "Failed handle event", http.StatusInternalServerError)
		requestsError.With(prometheus.Labels{"status_code": http.StatusText(http.StatusInternalServerError)}).Inc()
		return
	}

	w.WriteHeader(http.StatusOK)
}
