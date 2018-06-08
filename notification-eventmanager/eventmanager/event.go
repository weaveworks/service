package eventmanager

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"

	"github.com/weaveworks/blackfriday"
	"github.com/weaveworks/common/user"
	"github.com/weaveworks/service/notification-eventmanager/types"
	"github.com/weaveworks/service/users"
)

const (
	// MaxEventsList is the highest number of events that can be requested in one list call
	MaxEventsList        = 10000
	markdownNewline      = "  \n"
	markdownNewParagraph = "\n\n"
)

// slack URL like: <http://www.foo.com|foo.com>
var slackURL = regexp.MustCompile(`<([^|]+)?\|([^>]+)>`)

// handleCreateEvent handles event post requests and log them in DB and queue
func (em *EventManager) handleCreateEvent(r *http.Request, instanceID string) (interface{}, int, error) {
	defer r.Body.Close()
	requestsTotal.With(prometheus.Labels{"handler": "EventHandler"}).Inc()

	instanceID, _, err := user.ExtractOrgIDFromHTTPRequest(r)
	if err != nil {
		requestsError.With(prometheus.Labels{"status_code": http.StatusText(http.StatusUnauthorized)}).Inc()
		return nil, http.StatusUnauthorized, err
	}

	decoder := json.NewDecoder(r.Body)
	var e types.Event

	if err := decoder.Decode(&e); err != nil {
		requestsError.With(prometheus.Labels{"status_code": http.StatusText(http.StatusBadRequest)}).Inc()
		return nil, http.StatusBadRequest, errors.Wrap(err, "cannot decode event")
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
		requestsError.With(prometheus.Labels{"status_code": http.StatusText(http.StatusInternalServerError)}).Inc()
		return nil, http.StatusInternalServerError, errors.Wrap(err, "unable to retrieve instance data")
	}

	e.InstanceName = instanceData.Organization.Name

	eventID, err := em.storeAndSend(r.Context(), e, instanceData.Organization.FeatureFlags)

	if err != nil {
		requestsError.With(prometheus.Labels{"status_code": http.StatusText(http.StatusInternalServerError)}).Inc()
		return nil, http.StatusInternalServerError, errors.Wrap(err, "failed to handle event")
	}

	return struct {
		ID string `json:"id"`
	}{eventID}, http.StatusOK, nil
}

// handleTestEvent posts a test event for the user to verify things are working.
func (em *EventManager) handleTestEvent(r *http.Request, instanceID string) (interface{}, int, error) {
	defer r.Body.Close()
	requestsTotal.With(prometheus.Labels{"handler": "TestEventHandler"}).Inc()

	instanceData, err := em.UsersClient.GetOrganization(r.Context(), &users.GetOrganizationRequest{
		ID: &users.GetOrganizationRequest_InternalID{InternalID: instanceID},
	})

	if err != nil {
		if isStatusErrorCode(err, http.StatusNotFound) {
			requestsError.With(prometheus.Labels{"status_code": http.StatusText(http.StatusNotFound)}).Inc()
			return nil, http.StatusNotFound, err
		}
		requestsError.With(prometheus.Labels{"status_code": http.StatusText(http.StatusInternalServerError)}).Inc()
		return nil, http.StatusInternalServerError, errors.Wrap(err, "error requesting instance data from users service for test event")
	}

	if err != nil {
		requestsError.With(prometheus.Labels{"status_code": http.StatusText(http.StatusInternalServerError)}).Inc()
		return nil, http.StatusInternalServerError, errors.Wrap(err, "error getting stackdriver message for test event")
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
		requestsError.With(prometheus.Labels{"status_code": http.StatusText(http.StatusInternalServerError)}).Inc()
		return nil, http.StatusInternalServerError, errors.Wrap(err, "cannot post and send test event")
	}

	return eventID, http.StatusOK, nil
}

// handleSlackEvent handles slack json payload that includes the message text and some options, creates event, log it in DB and queue
func (em *EventManager) handleSlackEvent(r *http.Request) (interface{}, int, error) {
	requestsTotal.With(prometheus.Labels{"handler": "SlackHandler"}).Inc()
	vars := mux.Vars(r)

	eventType := vars["eventType"]
	if eventType == "" {
		eventsToSQSError.With(prometheus.Labels{"event_type": "empty"}).Inc()
		requestsError.With(prometheus.Labels{"status_code": http.StatusText(http.StatusBadRequest)}).Inc()
		return nil, http.StatusBadRequest, errors.New("eventType is empty in request")
	}
	log.Debugf("eventType = %s", eventType)

	instanceID := vars["instanceID"]
	if instanceID == "" {
		requestsError.With(prometheus.Labels{"status_code": http.StatusText(http.StatusBadRequest)}).Inc()
		return nil, http.StatusBadRequest, errors.New("instanceID is empty in request")
	}
	log.Debugf("instanceID = %s", instanceID)

	instanceData, err := em.UsersClient.GetOrganization(r.Context(), &users.GetOrganizationRequest{
		ID: &users.GetOrganizationRequest_InternalID{InternalID: instanceID},
	})
	if err != nil {
		if isStatusErrorCode(err, http.StatusNotFound) {
			requestsError.With(prometheus.Labels{"status_code": http.StatusText(http.StatusNotFound)}).Inc()
			return nil, http.StatusNotFound, errors.Wrap(err, "instance name not found")
		}
		return nil, http.StatusInternalServerError, errors.Wrap(err, "unable to retrieve instance data")
	}
	log.Debugf("Got data from users service: %v", instanceData.Organization.Name)

	defer r.Body.Close()
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		requestsError.With(prometheus.Labels{"status_code": http.StatusText(http.StatusBadRequest)}).Inc()
		return nil, http.StatusBadRequest, errors.Wrap(err, "cannot read body")
	}

	var sm types.SlackMessage
	if err = json.Unmarshal(body, &sm); err != nil {
		requestsError.With(prometheus.Labels{"status_code": http.StatusText(http.StatusBadRequest)}).Inc()
		return nil, http.StatusBadRequest, errors.Wrap(err, "cannot unmarshal body")
	}

	e, err := buildEvent(body, sm, eventType, instanceID, instanceData.Organization.Name)
	if err != nil {
		requestsError.With(prometheus.Labels{"status_code": http.StatusText(http.StatusBadRequest)}).Inc()
		return nil, http.StatusBadRequest, errors.Wrap(err, "cannot build event")
	}

	eventID, err := em.storeAndSend(r.Context(), e, instanceData.Organization.FeatureFlags)

	if err != nil {
		requestsError.With(prometheus.Labels{"status_code": http.StatusText(http.StatusInternalServerError)}).Inc()
		return nil, http.StatusInternalServerError, errors.Wrap(err, "failed to create event")
	}

	return struct {
		ID string `json:"id"`
	}{eventID}, http.StatusOK, nil
}

func (em *EventManager) handleGetEventTypes(r *http.Request) (interface{}, int, error) {
	result, err := em.DB.ListEventTypes(nil, getFeatureFlags(r))
	if err != nil {
		return nil, 0, err
	}
	return result, http.StatusOK, nil
}

// createConfigChangedEvent creates event with changed configuration
func (em *EventManager) createConfigChangedEvent(ctx context.Context, instanceID string, oldReceiver, receiver types.Receiver, eventTime time.Time, userEmail string, featureFlags []string) error {
	log.Debug("update_config Event Firing...")

	eventType := "config_changed"
	event := types.Event{
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

	sort.Strings(oldReceiver.EventTypes)
	sort.Strings(receiver.EventTypes)

	// currently only 3 events can happen on config/notifications page
	// 1) the address changed
	// 2) the eventTypes changed
	// 3) event fired unnecessarily, and niether address or eventTypes changed. (This happens because all receivers fire PUT change events on save)
	// Currently these are distinct non-overlapping events
	if !bytes.Equal(oldReceiver.AddressData, receiver.AddressData) {
		// address changed event
		msg := fmt.Sprintf("The address for <b>%s</b> was updated by %s!", receiver.RType, userEmail)

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

		event.Messages = map[string]json.RawMessage{
			types.EmailReceiver:       emailMsg,
			types.BrowserReceiver:     browserMsg,
			types.SlackReceiver:       json.RawMessage(fmt.Sprintf(`{"text": "*Instance:* %v\nThe address for *%s* was updated by %s!"}`, instanceName, receiver.RType, userEmail)),
			types.StackdriverReceiver: stackdriverMsg,
		}
	} else if !reflect.DeepEqual(oldReceiver.EventTypes, receiver.EventTypes) {
		// eventTypes changed event

		visibleEvents, err := em.DB.ListEventTypes(nil, featureFlags)

		if err != nil {
			return err
		}

		added, removed := diff(oldReceiver.EventTypes, receiver.EventTypes, visibleEvents)
		text := formatEventTypeText("<b>", "</b>", receiver.RType, "<i>", "</i>", added, removed, userEmail)

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

		slackText := formatEventTypeText("*", "*", receiver.RType, "_", "_", added, removed, userEmail)
		event.Messages = map[string]json.RawMessage{
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

		if _, err := em.storeAndSend(ctx, event, instanceData.Organization.FeatureFlags); err != nil {
			log.Warnf("failed to store in DB or send to SQS config change event")
		}
	}()

	return nil
}

func formatEventTypeText(rtypeStart, rtypeEnd, rtype, setStart, setEnd string, enabled, disabled []string, userEmail string) string {
	var b bytes.Buffer

	b.WriteString(fmt.Sprintf("The event types for %s%s%s were changed by %s:", rtypeStart, rtype, rtypeEnd, userEmail))
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

func isVisibleEvent(available []types.EventType, name string) bool {
	for _, et := range available {
		if !et.HideUIConfig && et.Name == name {
			return true
		}
	}

	return false
}

func diff(old, new []string, visibleEvents []types.EventType) (added []string, removed []string) {
	all := map[string]struct{}{}
	for _, et := range old {
		if isVisibleEvent(visibleEvents, et) {
			all[et] = struct{}{}
		}

	}

	for _, et := range new {
		if _, ok := all[et]; ok {
			delete(all, et)
		} else if isVisibleEvent(visibleEvents, et) {
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
		return nil, 0, err
	}

	return events, http.StatusOK, nil
}

func buildEvent(body []byte, sm types.SlackMessage, etype, instanceID, instanceName string) (types.Event, error) {
	var event types.Event
	allText := getAllMarkdownText(sm, instanceName)
	// handle slack URLs
	allTextMarkdownLinks := slackURL.ReplaceAllString(allText, "[$2]($1)")
	html := string(blackfriday.MarkdownBasic([]byte(allTextMarkdownLinks)))

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

// GetBrowserMessage returns messaage for browser
func getBrowserMessage(text string, attachments []types.SlackAttachment, etype string) (json.RawMessage, error) {
	bm := types.BrowserMessage{
		Type:        etype,
		Text:        text,
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
func getEmailMessage(text, etype, instanceName string) (json.RawMessage, error) {
	em := types.EmailMessage{
		Subject: fmt.Sprintf("%v - %v", instanceName, etype),
		Body:    text,
	}

	msgRaw, err := json.Marshal(em)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot marshal email message %s to json", em)
	}

	return msgRaw, nil
}

// GetStackdriverMessage returns message for stackdriver
func getStackdriverMessage(payload json.RawMessage, etype string, instanceName string) (json.RawMessage, error) {
	sdMsg := types.StackdriverMessage{
		Timestamp: time.Now(),
		Payload:   payload,
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
