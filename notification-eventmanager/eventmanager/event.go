package eventmanager

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"

	"github.com/weaveworks/common/user"
	"github.com/weaveworks/service/notification-eventmanager/types"
	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/notification-eventmanager/event"
)

const (
	// MaxEventsList is the highest number of events that can be requested in one list call
	MaxEventsList = 10000
)

// handleCreateEvent handles event post requests and log them in DB and queue
func (em *EventManager) handleCreateEvent(w http.ResponseWriter, r *http.Request) {
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

	eventID, err := em.storeAndSend(r.Context(), e, instanceData.Organization.FeatureFlags)

	if err != nil {
		log.Errorf("cannot post and send %s event, error: %s", e.Type, err)
		http.Error(w, "Failed to handle event", http.StatusInternalServerError)
		requestsError.With(prometheus.Labels{"status_code": http.StatusText(http.StatusInternalServerError)}).Inc()
		return
	}

	w.Write([]byte(eventID))
}

// handleTestEvent posts a test event for the user to verify things are working.
func (em *EventManager) handleTestEvent(w http.ResponseWriter, r *http.Request) {
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
		Type:         "user_test",
		InstanceID:   instanceID,
		InstanceName: instanceData.Organization.Name,
		Timestamp:    time.Now(),
		Text:         &text,
	}

	eventID, err := em.storeAndSend(r.Context(), testEvent, instanceData.Organization.FeatureFlags)

	if err != nil {
		log.Errorf("cannot post and send test event, error: %s", err)
		http.Error(w, "Failed to handle event", http.StatusInternalServerError)
		requestsError.With(prometheus.Labels{"status_code": http.StatusText(http.StatusInternalServerError)}).Inc()
		return
	}
	w.Write([]byte(eventID))
}

func (em *EventManager) handleGetEventTypes(r *http.Request) (interface{}, int, error) {
	result, err := em.DB.ListEventTypes(nil, getFeatureFlags(r))
	if err != nil {
		return nil, 0, err
	}
	return result, http.StatusOK, nil
}

// createConfigChangedEvent creates event with changed configuration
func (em *EventManager) createConfigChangedEvent(ctx context.Context, instanceID string, oldReceiver, receiver types.Receiver, eventTime time.Time) error {
	log.Debug("update_config Event Firing...")

	eventType := "config_changed"
	ev := types.Event{
		Type:       eventType,
		InstanceID: instanceID,
		Timestamp:  eventTime,
	}

	instanceData, err := em.UsersClient.GetOrganization(ctx, &users.GetOrganizationRequest{
		ID: &users.GetOrganizationRequest_InternalID{InternalID: instanceID},
	})
	if err != nil {
		if isStatusErrorCode(err, http.StatusNotFound) {
			log.Warnf("instance name for ID %s not found for event type %s", instanceID, eventType)
			return errors.Wrap(err, "instance not found")
		}
		log.Errorf("error requesting instance data from users service for event type %s: %s", eventType, err)
		return errors.Wrapf(err, "error requesting instance data from users service for event type %s", eventType)
	}
	instanceName := instanceData.Organization.Name
	ev.InstanceName = instanceName

	sort.Strings(oldReceiver.EventTypes)
	sort.Strings(receiver.EventTypes)

	// currently only 3 events can happen on config/notifications page
	// 1) the address changed
	// 2) the eventTypes changed
	// 3) event fired unnecessarily, and niether address or eventTypes changed. (This happens because all receivers fire PUT change events on save)
	// Currently these are distinct non-overlapping events
	if !bytes.Equal(oldReceiver.AddressData, receiver.AddressData) {
		// address changed event
		msg := fmt.Sprintf("The address for <b>%s</b> was updated!", receiver.RType)

		emailMsg, err := getEmailMessage(msg, eventType, instanceName)
		if err != nil {
			return errors.Wrap(err, "cannot get email message")
		}

		browserMsg, err := getBrowserMessage(msg, nil, eventType)
		if err != nil {
			return errors.Wrap(err, "cannot get email message")
		}

		msgJSON, err := json.Marshal(msg)
		if err != nil {
			return errors.Wrap(err, "cannot marshal message")
		}
		stackdriverMsg, err := getStackdriverMessage(msgJSON, eventType, instanceName)
		if err != nil {
			return errors.Wrap(err, "cannot get stackdriver message")
		}

		ev.Messages = map[string]json.RawMessage{
			types.EmailReceiver:       emailMsg,
			types.BrowserReceiver:     browserMsg,
			types.SlackReceiver:       json.RawMessage(fmt.Sprintf(`{"text": "*Instance:* %v\nThe address for *%s* was updated!"}`, instanceName, receiver.RType)),
			types.StackdriverReceiver: stackdriverMsg,
		}
	} else if !reflect.DeepEqual(oldReceiver.EventTypes, receiver.EventTypes) {
		// eventTypes changed event

		added, removed := diff(oldReceiver.EventTypes, receiver.EventTypes)
		text := formatEventTypeText("<b>", "</b>", receiver.RType, "<i>", "</i>", added, removed)

		emailMsg, err := getEmailMessage(text, eventType, instanceName)
		if err != nil {
			return errors.Wrap(err, "cannot get email message")
		}

		browserMsg, err := getBrowserMessage(text, nil, eventType)
		if err != nil {
			return errors.Wrap(err, "cannot get email message")
		}

		textJSON, err := json.Marshal(text)
		if err != nil {
			return errors.Wrap(err, "cannot marshal message")
		}
		stackdriverMsg, err := getStackdriverMessage(textJSON, eventType, instanceName)
		if err != nil {
			return errors.Wrap(err, "cannot get stackdriver message for event types changed")
		}

		slackText := formatEventTypeText("*", "*", receiver.RType, "_", "_", added, removed)
		ev.Data, err = json.Marshal(event.ConfigChangedData{
			Receiver: receiver.RType,
			Enabled:  added,
			Disabled: removed,
		})
		if err != nil {
			return errors.Wrap(err, "while marshalling data")
		}
		ev.Messages = map[string]json.RawMessage{
			types.EmailReceiver:   emailMsg,
			types.BrowserReceiver: browserMsg,
			// FIXME(rndstr): should use proper JSON marshalling here; e.g., this breaks for instance names with double quotes in them
			types.SlackReceiver:       json.RawMessage(fmt.Sprintf(`{"text": "*Instance:* %s\n%s"}`, instanceName, slackText)),
			types.StackdriverReceiver: stackdriverMsg,
		}
	} else {
		// nothing changed, don't send event
		return nil
	}

	go func() {

		if _, err := em.storeAndSend(ctx, ev, instanceData.Organization.FeatureFlags); err != nil {
			log.Warnf("failed to store in DB or send to SQS config change event")
		}
	}()

	return nil
}

func formatEventTypeText(rtypeStart, rtypeEnd, rtype, setStart, setEnd string, enabled, disabled []string) string {
	var b bytes.Buffer

	b.WriteString(fmt.Sprintf("The event types for %s%s%s were changed:", rtypeStart, rtype, rtypeEnd))
	if len(enabled) > 0 {
		b.WriteString(fmt.Sprintf(" enabled %s%s%s", setStart, enabled, setEnd))
	}
	if len(disabled) > 0 {
		if len(enabled) > 0 {
			b.WriteString(" and")
		}
		b.WriteString(fmt.Sprintf(" disabled %s%s%s", setStart, disabled, setEnd))

	}
	return b.String()
}

func diff(old, new []string) (added []string, removed []string) {
	all := map[string]struct{}{}
	for _, et := range old {
		all[et] = struct{}{}
	}

	for _, et := range new {
		if _, ok := all[et]; ok {
			delete(all, et)
		} else {
			added = append(added, et)
		}
	}

	for et := range all {
		removed = append(removed, et)
	}
	return
}

func (em *EventManager) handleGetEvents(r *http.Request, instanceID string) (interface{}, int, error) {
	var err error
	params := r.URL.Query()
	fields := []string{
		"event_id",
		"event_type",
		"instance_id",
		"timestamp",
		"messages",
		"text",
		"data",
		"metadata",
	}
	limit := 50
	offset := 0
	after := time.Unix(0, 0)
	before := time.Now().UTC()
	var eventTypes []string

	if params.Get("limit") != "" {
		l, err := strconv.Atoi(params.Get("limit"))
		if err != nil {
			return "Bad limit value: Not an integer", http.StatusBadRequest, nil
		}
		if l < 0 || l > MaxEventsList {
			return fmt.Sprintf("Bad limit value: Must be between 0 and %d inclusive", MaxEventsList), http.StatusBadRequest, nil
		}
		limit = l
	}
	if params.Get("offset") != "" {
		o, err := strconv.Atoi(params.Get("offset"))
		if err != nil {
			return "Bad offset value: Not an integer", http.StatusBadRequest, nil
		}
		if o < 0 {
			return "Bad offset value: Must be non-negative", http.StatusBadRequest, nil
		}
		offset = o
	}

	if params.Get("before") != "" {
		before, err = time.Parse(time.RFC3339Nano, params.Get("before"))
		if err != nil {
			return "Bad before value: Not an RFC3339 time", http.StatusBadRequest, nil
		}
	}
	if params.Get("after") != "" {
		after, err = time.Parse(time.RFC3339Nano, params.Get("after"))
		if err != nil {
			return "Bad after value:  Not an RFC3339 time", http.StatusBadRequest, nil
		}
	}
	if params.Get("event_type") != "" {
		eventTypes = strings.Split(params.Get("event_type"), ",")
	}
	if params.Get("fields") != "" {
		fieldsMap := map[string]struct{}{}
		for _, f := range fields {
			fieldsMap[f] = struct{}{}
		}

		newFields := strings.Split(params.Get("fields"), ",")
		for _, nf := range newFields {
			if _, ok := fieldsMap[nf]; !ok {
				return nil, http.StatusBadRequest, errors.Errorf("%s is an invalid field", nf)
			}
		}

		fields = newFields
	}

	events, err := em.DB.GetEvents(instanceID, fields, eventTypes, before, after, limit, offset)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	// Process new event format by rendering its data for the browser
	for _, ev := range events {
		if len(ev.Data) > 0 {
			ev.Messages = nil
			text := em.types.Render(Browser, ev).Text()
			if err != nil {
				return nil, http.StatusInternalServerError, err
			}
			ev.Text = &text
		}
	}

	return events, http.StatusOK, nil
}

// storeAndSend stores event in DB and sends notification batches for this event to SQS
func (em *EventManager) storeAndSend(ctx context.Context, ev types.Event, featureFlags []string) (string, error) {
	eventID, err := em.DB.CreateEvent(ev, featureFlags)
	if err != nil {
		eventsToDBError.With(prometheus.Labels{"event_type": ev.Type}).Inc()
		return "", errors.Wrapf(err, "cannot store event in DB")
	}
	eventsToDBTotal.With(prometheus.Labels{"event_type": ev.Type}).Inc()

	ev.ID = eventID
	if err := em.sendNotificationBatchesToQueue(ctx, ev); err != nil {
		eventsToSQSError.With(prometheus.Labels{"event_type": ev.Type}).Inc()
		return "", errors.Wrapf(err, "cannot send notification batches to queue")
	}
	eventsToSQSTotal.With(prometheus.Labels{"event_type": ev.Type}).Inc()

	return eventID, nil
}
