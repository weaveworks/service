package eventmanager

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/aws-sdk-go/service/sqs/sqsiface"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
	"google.golang.org/grpc/status"

	"github.com/weaveworks/blackfriday"
	"github.com/weaveworks/common/user"
	"github.com/weaveworks/service/notification-eventmanager/types"
	"github.com/weaveworks/service/notification-eventmanager/utils"
	"github.com/weaveworks/service/notification-sender"
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

	databaseRequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "notification",
		Name:      "database_request_duration_seconds",
		Help:      "Time spent (in seconds) doing database requests.",
		Buckets:   prometheus.DefBuckets,
	}, []string{"method", "status_code"})
)

// EventManager contains Users service client, DB connection and SQS queue to store events into DB and send notifications to SQS queue
type EventManager struct {
	UsersClient users.UsersClient
	DB          *utils.DB
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
		databaseRequestDuration,
	)
}

// New creates new EventManager
func New(usersClient users.UsersClient, db *sql.DB, sqsClient sqsiface.SQSAPI, sqsQueue string) *EventManager {
	return &EventManager{
		UsersClient: usersClient,
		DB:          utils.NewDB(db, databaseRequestDuration),
		SQSClient:   sqsClient,
		SQSQueue:    sqsQueue,
		limiter:     rate.NewLimiter(ratelimit, ratelimit),
	}
}

// Register HTTP handlers
func (em *EventManager) Register(r *mux.Router) {
	for _, route := range []struct {
		name, method, path string
		handler            http.Handler
	}{
		{"list_event_types", "GET", "/api/notification/config/eventtypes", withNoArgs(em.httpListEventTypes)},
		{"list_receivers", "GET", "/api/notification/config/receivers", withInstance(em.listReceivers)},
		{"create_receiver", "POST", "/api/notification/config/receivers", withInstance(em.httpCreateReceiver)},
		{"get_receiver", "GET", "/api/notification/config/receivers/{id}", withInstanceAndID(em.getReceiver)},
		{"update_receiver", "PUT", "/api/notification/config/receivers/{id}", withInstanceAndID(em.updateReceiver)},
		{"delete_receiver", "DELETE", "/api/notification/config/receivers/{id}", withInstanceAndID(em.deleteReceiver)},
		{"get_events", "GET", "/api/notification/events", withInstance(em.getEvents)},

		// Internal API
		{"create_test_event", "POST", "/api/notification/testevent", em.RateLimited(em.TestEventHandler)},
		{"create_event", "POST", "/api/notification/events", em.RateLimited(em.EventHandler)},
		{"create_slack_event", "POST", "/api/notification/slack/{instanceID}/{eventType}", em.RateLimited(em.SlackHandler)},
		{"health_check", "GET", "/api/notification/events/healthcheck", http.HandlerFunc(em.HandleHealthCheck)},

		// External API - reachable from outside Weave Cloud cluster
		{"create_event_external", "POST", "/api/notification/external/events", em.RateLimited(em.EventHandler)},
	} {
		r.Handle(route.path, route.handler).Methods(route.method).Name(route.name)
	}
}

// Wait waits until all SQS send requests are finished
func (em *EventManager) Wait() {
	em.wg.Wait()
}

// SendNotificationBatchesToQueue gets notifications for event, partitions them to batches and sends to SQS queue
func (em *EventManager) SendNotificationBatchesToQueue(ctx context.Context, e types.Event) error {
	notifs, err := em.getNotifications(ctx, e)
	if err != nil {
		return errors.Wrapf(err, "cannot get all notifications for event %v", e)
	}

	notifBatches := partitionNotifications(notifs, batchSize)
	for _, batch := range notifBatches {
		sendInp, err := em.notificationBatchToSendInput(batch)
		if err != nil {
			return errors.Wrap(err, "cannot get SQS send input for notification batch")
		}

		em.wg.Add(1)
		go func() {
			defer em.wg.Done()
			_, err = em.SQSClient.SendMessageBatch(sendInp)
			if err != nil {
				log.Errorf("cannot send to SQS queue batch input, error: %s", err)
				eventsToSQSError.With(prometheus.Labels{"event_type": e.Type}).Inc()
				return
			}
			sender.NotificationsInSQS.Add(float64(len(notifs)))
		}()
	}

	return nil
}

func (em *EventManager) getNotifications(ctx context.Context, e types.Event) ([]types.Notification, error) {
	receivers, err := em.GetReceiversForEvent(e)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot get receivers for event %v", e)
	}
	log.Debugf("Got %d receivers for InstanceID = %s and event type = %s", len(receivers), e.InstanceID, e.Type)

	var notifications []types.Notification
	for _, r := range receivers {
		notif := types.Notification{
			ReceiverType: r.RType,
			InstanceID:   e.InstanceID,
			Address:      r.AddressData,
			Data:         e.Messages[r.RType],
			Event:        e,
		}
		notifications = append(notifications, notif)
	}

	return notifications, nil
}

// partitionNotifications takes slice of notifications, partitions it to batches with size batchSize
// and returns slice of slices of notifications
func partitionNotifications(notifs []types.Notification, batchSize int) [][]types.Notification {
	var batch []types.Notification
	var notifBatches [][]types.Notification

	for len(notifs) >= batchSize {
		batch, notifs = notifs[:batchSize], notifs[batchSize:]
		notifBatches = append(notifBatches, batch)
	}

	if len(notifs) > 0 {
		notifBatches = append(notifBatches, notifs)
	}

	return notifBatches
}

func (em *EventManager) notificationBatchToSendInput(batch []types.Notification) (*sqs.SendMessageBatchInput, error) {
	var entries []*sqs.SendMessageBatchRequestEntry
	for i, notif := range batch {
		notifStr, err := notificationToString(notif)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot marshal notification %s to string", notif)
		}
		entry := &sqs.SendMessageBatchRequestEntry{
			Id:          aws.String(strconv.Itoa(i)),
			MessageBody: aws.String(notifStr),
		}
		entries = append(entries, entry)
	}
	return &sqs.SendMessageBatchInput{
		Entries:  entries,
		QueueUrl: &em.SQSQueue,
	}, nil
}

func notificationToString(n types.Notification) (string, error) {
	raw, err := json.Marshal(n)
	if err != nil {
		return "", errors.Wrapf(err, "cannot marshal notification %s", n)
	}

	return string(raw), nil
}

// isStatusErrorCode returns true if the error has the given status code.
func isStatusErrorCode(err error, code int) bool {
	st, ok := status.FromError(err)
	if !ok {
		return false
	}
	return code == int(st.Code())
}

// TestEventHandler creates test event
func (em *EventManager) TestEventHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	requestsTotal.With(prometheus.Labels{"handler": "TestEventHandler"}).Inc()
	instanceID, _, err := user.ExtractOrgIDFromHTTPRequest(r)
	if err != nil {
		log.Errorf("cannot create test event, failed to extract orgID, error: %s", err)
		http.Error(w, err.Error(), http.StatusUnauthorized)
		requestsError.With(prometheus.Labels{"status_code": http.StatusText(http.StatusUnauthorized)}).Inc()
		return
	}

	instanceData, err := em.UsersClient.GetOrganization(r.Context(), &users.GetOrganizationRequest{
		ID: &users.GetOrganizationRequest_InternalID{InternalID: instanceID},
	})

	if err != nil {
		if isStatusErrorCode(err, http.StatusNotFound) {
			log.Warnf("instance name for ID %s not found for test event", instanceID)
			http.Error(w, "Instance not found", http.StatusNotFound)
			requestsError.With(prometheus.Labels{"status_code": http.StatusText(http.StatusNotFound)}).Inc()
			return
		}
		log.Errorf("error requesting instance data from users service for test event: %s", err)
		http.Error(w, "unable to retrieve instance data", http.StatusInternalServerError)
		requestsError.With(prometheus.Labels{"status_code": http.StatusText(http.StatusInternalServerError)}).Inc()
		return
	}

	if err != nil {
		log.Errorf("error getting stackdriver message for test event: %s", err)
		http.Error(w, "unable to get stackdriver message", http.StatusInternalServerError)
		requestsError.With(prometheus.Labels{"status_code": http.StatusText(http.StatusInternalServerError)}).Inc()
		return
	}

	text := "A test event triggered from Weave Cloud!"

	testEvent := types.Event{
		Type:       "user_test",
		InstanceID: instanceID,
		Timestamp:  time.Now(),
		Text:       &text,
		Metadata:   map[string]string{"instance_name": instanceData.Organization.Name},
	}

	if err := em.storeAndSend(r.Context(), testEvent); err != nil {
		log.Errorf("cannot post and send test event, error: %s", err)
		http.Error(w, "Failed handle event", http.StatusInternalServerError)
		requestsError.With(prometheus.Labels{"status_code": http.StatusText(http.StatusInternalServerError)}).Inc()
		return
	}

	w.WriteHeader(http.StatusOK)
}

// RateLimited is rate limit middleware
func (em *EventManager) RateLimited(next http.HandlerFunc) http.HandlerFunc {
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

// EventHandler handles event post requests and log them in DB and queue
func (em *EventManager) EventHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	requestsTotal.With(prometheus.Labels{"handler": "EventHandler"}).Inc()

	instanceID, _, err := user.ExtractOrgIDFromHTTPRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		requestsError.With(prometheus.Labels{"status_code": http.StatusText(http.StatusUnauthorized)}).Inc()
		return
	}

	decoder := json.NewDecoder(r.Body)
	var e types.Event

	if err := decoder.Decode(&e); err != nil {
		requestsError.With(prometheus.Labels{"status_code": http.StatusText(http.StatusBadRequest)}).Inc()
		log.Errorf("cannot decode event, error: %s", err)
		http.Error(w, "Cannot decode event", http.StatusBadRequest)
		return
	}

	// Override if InstanceID is undefined.
	// Events from the weave cloud ui do not popuplate an InstanceID in the POST body.
	// instanceID is the internal integer identifier, not the user-facing instanceID.
	if e.InstanceID == "" {
		e.InstanceID = instanceID
	}

	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now()
	}

	instanceData, err := em.UsersClient.GetOrganization(r.Context(), &users.GetOrganizationRequest{
		ID: &users.GetOrganizationRequest_InternalID{InternalID: instanceID},
	})

	if err != nil {
		log.Errorf("error requesting instance data from users service for event type %s: %s", e.Type, err)
		http.Error(w, "unable to retrieve instance data", http.StatusInternalServerError)
		return
	}

	e.InstanceName = instanceData.Organization.Name

	if err := em.storeAndSend(r.Context(), e); err != nil {
		log.Errorf("cannot post and send %s event, error: %s", e.Type, err)
		http.Error(w, "Failed handle event", http.StatusInternalServerError)
		requestsError.With(prometheus.Labels{"status_code": http.StatusText(http.StatusInternalServerError)}).Inc()
		return
	}

	w.WriteHeader(http.StatusOK)
}

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

	if err := em.storeAndSend(r.Context(), e); err != nil {
		log.Errorf("cannot post and send %s event, error: %s", e.Type, err)
		http.Error(w, "Failed handle event", http.StatusInternalServerError)
		requestsError.With(prometheus.Labels{"status_code": http.StatusText(http.StatusInternalServerError)}).Inc()
		return
	}

	w.WriteHeader(http.StatusOK)
}

// storeAndSend stores event in DB and sends notification batches for this event to SQS
func (em *EventManager) storeAndSend(ctx context.Context, ev types.Event) error {
	if err := em.CreateEvent(ev); err != nil {
		eventsToDBError.With(prometheus.Labels{"event_type": ev.Type}).Inc()
		return errors.Wrapf(err, "cannot store event in DB")
	}
	eventsToDBTotal.With(prometheus.Labels{"event_type": ev.Type}).Inc()

	if err := em.SendNotificationBatchesToQueue(ctx, ev); err != nil {
		eventsToSQSError.With(prometheus.Labels{"event_type": ev.Type}).Inc()
		return errors.Wrapf(err, "cannot send notification batches to queue")
	}
	eventsToSQSTotal.With(prometheus.Labels{"event_type": ev.Type}).Inc()

	return nil
}

func buildEvent(body []byte, sm types.SlackMessage, etype, instanceID, instanceName string) (types.Event, error) {
	var event types.Event
	allText := getAllMarkdownText(sm, instanceName)
	html := string(blackfriday.MarkdownBasic([]byte(allText)))

	emailMsg, err := getEmailMessage(html, etype, instanceName)
	if err != nil {
		return event, errors.Wrap(err, "cannot get email message")
	}

	browserMsg, err := getBrowserMessage(sm.Text, sm.Attachments, etype)
	if err != nil {
		return event, errors.Wrap(err, "cannot get email message")
	}

	stackdriverMsg, err := getStackdriverMessage(json.RawMessage(body), etype, instanceName)
	if err != nil {
		return event, errors.Wrap(err, "cannot get stackdriver message")
	}

	sm.Text = fmt.Sprintf("*Instance*: %v\n%s", instanceName, sm.Text)

	slackMsg, err := json.Marshal(sm)
	if err != nil {
		return event, errors.Wrap(err, "cannot get slack message")
	}

	event.InstanceID = instanceID
	event.Type = etype
	event.Timestamp = time.Now()
	event.Messages = map[string]json.RawMessage{
		types.BrowserReceiver:     browserMsg,
		types.SlackReceiver:       slackMsg,
		types.EmailReceiver:       emailMsg,
		types.StackdriverReceiver: stackdriverMsg,
	}

	return event, nil
}

// GetBrowserMessage returns messaage for browser
func getBrowserMessage(msg string, attachments []types.SlackAttachment, etype string) (json.RawMessage, error) {
	bm := types.BrowserMessage{
		Type:        etype,
		Text:        msg,
		Attachments: attachments,
		Timestamp:   time.Now(),
	}

	msgRaw, err := json.Marshal(bm)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot marshal browser message %s to json", bm)
	}

	return msgRaw, nil
}

// GetEmailMessage returns message for email
func getEmailMessage(msg, etype, instanceName string) (json.RawMessage, error) {
	em := types.EmailMessage{
		Subject: fmt.Sprintf("%v - %v", instanceName, etype),
		Body:    msg,
	}

	msgRaw, err := json.Marshal(em)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot marshal email message %s to json", em)
	}

	return msgRaw, nil
}

// GetStackdriverMessage returns message for stackdriver
func getStackdriverMessage(msg json.RawMessage, etype string, instanceName string) (json.RawMessage, error) {
	sdMsg := types.StackdriverMessage{
		Timestamp: time.Now(),
		Payload:   msg,
		Labels:    map[string]string{"instance": instanceName, "event_type": etype},
	}

	msgRaw, err := json.Marshal(sdMsg)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot marshal stackdriver message %s to json", sdMsg)
	}

	return msgRaw, nil
}

// GetAllMarkdownText returns all text in markdown format from slack message (text and attachments)
func getAllMarkdownText(sm types.SlackMessage, instanceName string) string {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("Instance: %s%s", instanceName, markdownNewParagraph))
	if sm.Text != "" {
		// a slack message might contain \n for new lines
		// replace it with markdown line break
		buf.WriteString(strings.Replace(sm.Text, "\n", markdownNewline, -1))
		buf.WriteString(markdownNewParagraph)
	}
	for _, att := range sm.Attachments {
		if att.Pretext != "" {
			buf.WriteString(strings.Replace(att.Pretext, "\n", markdownNewline, -1))
			buf.WriteString(markdownNewline)
		}
		if att.Title != "" {
			buf.WriteString(strings.Replace(att.Title, "\n", markdownNewline, -1))
			buf.WriteString(markdownNewline)
		}
		if att.Text != "" {
			buf.WriteString(strings.Replace(att.Text, "\n", markdownNewline, -1))
		}
		buf.WriteString(markdownNewParagraph)
	}

	return buf.String()
}
