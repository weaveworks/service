package sender

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/aws-sdk-go/service/sqs/sqsiface"
	"github.com/gorilla/websocket"
	nats "github.com/nats-io/go-nats"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"github.com/weaveworks/common/user"
	"github.com/weaveworks/service/notification-configmanager/types"
)

// RetriableError is error after sending notification when sender will retry sending again
// otherwise error is not retriable and sender will log error, increment appropriate metric and return nil error after send
type RetriableError struct {
	error
}

var (
	receiveFromSQS = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "notifications_receive_from_sqs_total",
		Help: "Total number of notifications read from SQS."})

	receiveFromSQSError = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "notification_receive_from_sqs_errors_total",
		Help: "Total number of failures when reading from SQS."})

	deleteFromSQS = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "notifications_delete_from_sqs_total",
		Help: "Total number of notifications deleted from SQS.",
	}, []string{"receiver_type"})

	notificationsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "notifications_total",
		Help: "Total number of notifications in sender service.",
	}, []string{"receiver_type"})

	notificationsError = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "notification_errors_total",
		Help: "Total number of errors in notifications in sender service.",
	}, []string{"receiver_type"})

	// NotificationsInSQS is number of notifications are still active currently in SQS
	NotificationsInSQS = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "notifications_in_sqs",
		Help: "Total number of notifications are still active currently in SQS."})

	publicationsNATS = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "publications_nats_total",
		Help: "Total number of browser notification publications to NATS."})

	publicationsNATSErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "publication_nats_errors_total",
		Help: "Total number of errors publishing browser notification to NATS."})

	subscribersNATS = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "subscribers_nats",
		Help: "Total number of browsers still currently subscribed to NATS."})

	subscribersNATSErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "subscribers_nats_errors_total",
		Help: "Total number of errors subscribing to NATS or updating NATS."})

	notificationsSentToBrowserTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "notifications_to_browser_total",
		Help: "Total number of notifications sent over websocket to browers."})

	notificationsSentToBrowserErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "notification_to_browser_errors_total",
		Help: "Total number of failures when sending over websocket"})

	noRetryError = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "notification_no_retry_errors_total",
		Help: "Total number of errors sending to notification where the request will not be retried without correction."})

	websocketsActive = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "notification_active_websockets",
		Help: "Total number of websockets are currently open / connected."})

	incomingWebsocketsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "notification_websockets_total",
		Help: "Total number of incoming websocket connections."})

	websocketErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "notification_websocket_errors_total",
		Help: "Total number of errors in websocket connection."})

	completedDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: "notification",
		Name:      "completed_notification_duration_seconds",
		Help:      "Histogram for the completed notification duration.",
		Buckets:   prometheus.DefBuckets,
	})
)

const (
	// Send pings to peer with this period
	pingPeriod = 30 * time.Second

	// Time allowed to write a message to the peer
	writeWait = 10 * time.Second
)

// Config for sender contains interface for SQS API and SQS queue name
type Config struct {
	SQSCli   sqsiface.SQSAPI
	SQSQueue string
	NATS     *nats.Conn
}

// NotifyHandler notifies address about notification in data
type NotifyHandler func(ctx context.Context, address, data json.RawMessage, instance string) error

// Sender contains creds for SQS and map of handlers to each ReceiverType
type Sender struct {
	SQSCli         sqsiface.SQSAPI
	SQSQueue       string
	NATS           *nats.Conn
	notifyHandlers map[string]NotifyHandler
	wg             sync.WaitGroup
}

func init() {
	prometheus.MustRegister(
		receiveFromSQS,
		receiveFromSQSError,
		deleteFromSQS,
		notificationsTotal,
		notificationsError,
		NotificationsInSQS,
		publicationsNATS,
		publicationsNATSErrors,
		subscribersNATS,
		subscribersNATSErrors,
		notificationsSentToBrowserTotal,
		notificationsSentToBrowserErrors,
		noRetryError,
		websocketsActive,
		websocketErrors,
		incomingWebsocketsTotal,
		completedDuration,
	)
}

// New creates new Sender with info from Config
func New(c Config) *Sender {
	return &Sender{
		SQSCli:         c.SQSCli,
		SQSQueue:       c.SQSQueue,
		NATS:           c.NATS,
		notifyHandlers: make(map[string]NotifyHandler),
	}
}

// Run is waiting for new notifications from SQS queue, use context ctx to cancel sender
func (s *Sender) Run(ctx context.Context) {
	defer s.wg.Wait()
	for {
		if err := s.HandleNotification(ctx); err != nil {
			if err == context.Canceled {
				return
			}
			log.Errorf("Handle notification error: %s", err)
		}
	}
}

// RegisterNotifier registers handler function for handlerType
func (s *Sender) RegisterNotifier(handlerType string, h NotifyHandler) {
	s.notifyHandlers[handlerType] = h
}

// HandleNotification receives notification from queue,
// convert it to string and starts new go routine to send this notification
func (s *Sender) HandleNotification(ctx context.Context) error {
	recvInp := &sqs.ReceiveMessageInput{
		WaitTimeSeconds:   aws.Int64(20),
		VisibilityTimeout: aws.Int64(60),
		QueueUrl:          &s.SQSQueue,
	}

	out, err := s.SQSCli.ReceiveMessageWithContext(ctx, recvInp)
	if err != nil {
		aerr, ok := err.(awserr.Error)
		if ok && aerr.OrigErr() == context.Canceled {
			return context.Canceled
		}
		receiveFromSQSError.Inc()
		return errors.Wrapf(err, "cannot receive notification from SQS")

	}

	if len(out.Messages) == 0 {
		return nil
	}

	// always receive only 1 message by using default value of MaxNumberOfMessages in ReceiveMessageInput
	// Valid values are 1 to 10. Default is 1.
	receiveFromSQS.Inc()
	msg := out.Messages[0]
	notif, err := stringToNotification(*msg.Body)
	if err != nil {
		return err
	}

	s.wg.Add(1)
	notificationsTotal.With(prometheus.Labels{"receiver_type": notif.ReceiverType}).Inc()
	go func() {
		defer s.wg.Done()
		if err = s.sendNotification(ctx, notif, msg.ReceiptHandle); err != nil {
			log.Errorf("cannot send notification; error %s", err)
			notificationsError.With(prometheus.Labels{"receiver_type": notif.ReceiverType}).Inc()
			return
		}
	}()

	return nil
}

func stringToNotification(msg string) (types.Notification, error) {
	var notif types.Notification
	if err := json.Unmarshal([]byte(msg), &notif); err != nil {
		return notif, errors.Wrapf(err, "cannot unmarshal json string %v to notification", msg)
	}
	return notif, nil
}

// sendNotification tries to sends notification,
// if successful or error is not RetriableError then delete this notification from queue
// otherwise this notification will be available again after VisibilityTimeout
func (s *Sender) sendNotification(ctx context.Context, notif types.Notification, receiptHandle *string) error {
	h, ok := s.notifyHandlers[notif.ReceiverType]
	if !ok {
		return errors.Errorf("no handler for type %s", notif.ReceiverType)
	}

	begin := time.Now()
	if err := h(ctx, notif.Address, notif.Data, notif.InstanceID); err != nil {
		if _, ok := err.(RetriableError); ok {
			return errors.Wrapf(err, "cannot send %s notification; will retry request later, retriable error: %s", notif.ReceiverType, err)
		}
		// Warning because it must be user error and shouldn't trigger the alerts
		log.Warnf("cannot send %s notification; the request will not be retried without correction; error: %s", notif.ReceiverType, err)
		noRetryError.Inc()
	}
	completedDuration.Observe(time.Since(begin).Seconds())

	deleteInp := &sqs.DeleteMessageInput{
		QueueUrl:      &s.SQSQueue,
		ReceiptHandle: receiptHandle,
	}
	if _, err := s.SQSCli.DeleteMessage(deleteInp); err != nil {
		return errors.Wrapf(err, "failed to delete message from SQS")
	}
	deleteFromSQS.With(prometheus.Labels{"receiver_type": notif.ReceiverType}).Inc()
	NotificationsInSQS.Dec()

	return nil
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// HandleBrowserNotifications handles browser requests for notifications (websocket)
func (s *Sender) HandleBrowserNotifications(w http.ResponseWriter, r *http.Request) {
	log.Debugf("new websocket connection from %s", r.UserAgent())
	incomingWebsocketsTotal.Inc()
	websocketsActive.Inc()
	instance, _, err := user.ExtractOrgIDFromHTTPRequest(r)
	if err != nil {
		log.Errorf("cannot extract OrgID, error: %s\n", err)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		websocketsActive.Dec()
		websocketErrors.Inc()
		return
	}

	ch := make(chan *nats.Msg, 64)
	sub, err := s.NATS.ChanSubscribe(instance, ch)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		log.Errorf("cannot subscribe to NATS, error: %s\n", err)
		websocketsActive.Dec()
		subscribersNATSErrors.Inc()
		return
	}
	log.Debugf("NATS subscribed to instance = %s", instance)
	subscribersNATS.Inc()

	defer func() {
		if err := sub.Unsubscribe(); err != nil {
			log.Errorf("NATS cannot unsubscribe %s\n", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			subscribersNATSErrors.Inc()
		}
		log.Debugf("NATS unsubscribed from instance = %s", instance)
		subscribersNATS.Dec()
	}()

	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		log.Errorf("cannot upgrade websocket, error: %s\n", err)
		websocketsActive.Dec()
		websocketErrors.Inc()
		return
	}
	defer c.Close()

	closed := make(chan struct{})
	go func() {
		for {
			_, _, err := c.ReadMessage()
			if err != nil {
				log.Debugf("cannot read from websocket: %s, close connection", err)
				close(closed)
				return
			}
		}
	}()

	ticker := time.NewTicker(pingPeriod)

	for {
		select {
		case msg := <-ch:
			if err := c.WriteMessage(websocket.TextMessage, append(msg.Data, '\n')); err != nil {
				log.Errorf("cannot write message to websocket, error: %s", err)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				notificationsSentToBrowserErrors.Inc()
				return
			}
			log.Debugf("wrote new message to websocket: %s", msg.Data)
			notificationsSentToBrowserTotal.Inc()
		case <-ticker.C:
			if err := c.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(writeWait)); err != nil {
				websocketsActive.Dec()
				return
			}
		case <-closed:
			log.Debug("websocket closed")
			websocketsActive.Dec()
			return
		}
	}
}

// HandleHealthCheck handles a very simple health check
func (s *Sender) HandleHealthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}
