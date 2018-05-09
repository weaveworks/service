package eventmanager

import (
	"net/http"
	"sync"

	"github.com/aws/aws-sdk-go/service/sqs/sqsiface"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/time/rate"
	_ "gopkg.in/mattes/migrate.v1/driver/postgres" // Import the postgres migrations driver

	"github.com/weaveworks/service/notification-eventmanager/db"
	"github.com/weaveworks/service/users"
)

const ratelimit = 100

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

// handleHealthCheck handles a very simple health check
func (em *EventManager) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}
