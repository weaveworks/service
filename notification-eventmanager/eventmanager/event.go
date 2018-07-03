package eventmanager

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"

	"github.com/weaveworks/common/user"
	"github.com/weaveworks/service/notification-eventmanager/eventmanager/parser"
	"github.com/weaveworks/service/notification-eventmanager/types"
	"github.com/weaveworks/service/users"
)

const (
	// MaxEventsList is the highest number of events that can be requested in one list call
	MaxEventsList          = 10000
	notificationConfigPath = "/org/notifications"
	alertsPage             = "/prom/alerts"
	alertLinkText          = "Weave Cloud Alert"
	deployPage             = "/deploy/services"
	deployLinkText         = "Weave Cloud Deploy"
)

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

	instanceName := instanceData.Organization.Name
	etype := "user_test"
	text := "A test event triggered from Weave Cloud!"
	textJSON, err := json.Marshal(text)
	if err != nil {
		return nil, http.StatusInternalServerError, errors.Wrapf(err, "cannot marshal text: %s", text)
	}

	sdMsg, err := parser.StackdriverFromSlack(textJSON, etype, instanceName)
	if err != nil {
		requestsError.With(prometheus.Labels{"status_code": http.StatusText(http.StatusInternalServerError)}).Inc()
		return nil, http.StatusInternalServerError, errors.Wrap(err, "error getting stackdriver message for test event")
	}

	link, err := em.getInstanceLink(instanceData.Organization.ExternalID, notificationConfigPath)
	if err != nil {
		requestsError.With(prometheus.Labels{"status_code": http.StatusText(http.StatusInternalServerError)}).Inc()
		return nil, http.StatusInternalServerError, errors.Wrap(err, "error getting notification config page link for test event")
	}

	emailMsg, err := parser.EmailFromSlack(text, etype, instanceName, link)
	if err != nil {
		requestsError.With(prometheus.Labels{"status_code": http.StatusText(http.StatusInternalServerError)}).Inc()
		return nil, http.StatusInternalServerError, errors.Wrap(err, "error getting email message for test event")
	}

	slackMsg, err := parser.SlackFromSlack(types.SlackMessage{Text: text}, instanceName, link)
	if err != nil {
		requestsError.With(prometheus.Labels{"status_code": http.StatusText(http.StatusInternalServerError)}).Inc()
		return nil, http.StatusInternalServerError, errors.Wrap(err, "error getting slack text message for test event")
	}

	browserMsg, err := parser.BrowserFromSlack(types.SlackMessage{Text: text}, etype, "", "")
	if err != nil {
		requestsError.With(prometheus.Labels{"status_code": http.StatusText(http.StatusInternalServerError)}).Inc()
		return nil, http.StatusInternalServerError, errors.Wrap(err, "error getting browser message for test event")
	}

	opsGenieMsg, err := parser.OpsGenieFromSlack(text, etype, instanceName)
	if err != nil {
		requestsError.With(prometheus.Labels{"status_code": http.StatusText(http.StatusInternalServerError)}).Inc()
		return nil, http.StatusInternalServerError, errors.Wrap(err, "error getting OpsGenie message for test event")
	}

	testEvent := types.Event{
		Type:         etype,
		InstanceID:   instanceID,
		InstanceName: instanceData.Organization.Name,
		Timestamp:    time.Now(),
		Messages: map[string]json.RawMessage{
			types.EmailReceiver:       emailMsg,
			types.BrowserReceiver:     browserMsg,
			types.SlackReceiver:       slackMsg,
			types.StackdriverReceiver: sdMsg,
			types.OpsGenieReceiver:    opsGenieMsg,
		},
	}

	eventID, err := em.storeAndSend(r.Context(), testEvent, instanceData.Organization.FeatureFlags)

	if err != nil {
		requestsError.With(prometheus.Labels{"status_code": http.StatusText(http.StatusInternalServerError)}).Inc()
		return nil, http.StatusInternalServerError, errors.Wrap(err, "cannot post and send test event")
	}

	return eventID, http.StatusOK, nil
}

// handleWebhookEvent handles webhook json payload, creates event, log it in DB and queue
func (em *EventManager) handleWebhookEvent(r *http.Request) (interface{}, int, error) {
	requestsTotal.With(prometheus.Labels{"handler": "handleWebhookEvent"}).Inc()
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

	var wm types.WebhookAlert
	if err = json.Unmarshal(body, &wm); err != nil {
		requestsError.With(prometheus.Labels{"status_code": http.StatusText(http.StatusBadRequest)}).Inc()
		return nil, http.StatusBadRequest, errors.Wrap(err, "cannot unmarshal body")
	}

	notifLink, err := em.getInstanceLink(instanceData.Organization.ExternalID, notificationConfigPath)
	if err != nil {
		requestsError.With(prometheus.Labels{"status_code": http.StatusText(http.StatusBadRequest)}).Inc()
		return nil, http.StatusBadRequest, errors.Wrap(err, "cannot get Weave Cloud notification page link")
	}

	var linkText, linkPath string
	// link to Monitor page with Firing alerts for Cortex events
	switch eventType {
	case "monitor":
		linkText = alertLinkText
		linkPath = alertsPage
	default:
		requestsError.With(prometheus.Labels{"status_code": http.StatusText(http.StatusBadRequest)}).Inc()
		return nil, http.StatusBadRequest, errors.Wrap(err, "wrong event type, a monitor is expected")
	}

	link, err := em.getInstanceLink(instanceData.Organization.ExternalID, linkPath)
	if err != nil {
		requestsError.With(prometheus.Labels{"status_code": http.StatusText(http.StatusBadRequest)}).Inc()
		return nil, http.StatusBadRequest, errors.Wrap(err, "cannot get Weave Cloud deploy page link")
	}

	e, err := buildWebhookEvent(wm, eventType, instanceID, instanceData.Organization.Name, notifLink, link, linkText)
	if err != nil {
		requestsError.With(prometheus.Labels{"status_code": http.StatusText(http.StatusBadRequest)}).Inc()
		return nil, http.StatusBadRequest, errors.Wrap(err, "cannot build webhook event")
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

	notifLink, err := em.getInstanceLink(instanceData.Organization.ExternalID, notificationConfigPath)
	if err != nil {
		requestsError.With(prometheus.Labels{"status_code": http.StatusText(http.StatusBadRequest)}).Inc()
		return nil, http.StatusBadRequest, errors.Wrap(err, "cannot get Weave Cloud notification page link")
	}

	var linkText, linkPath string
	// link to Monitor page with Firing alerts for Cortex events
	// and link to Deploy page for Flux events
	switch eventType {
	case "monitor":
		linkText = alertLinkText
		linkPath = alertsPage
	case "sync",
		"policy",
		"deploy",
		"auto_deploy",
		"deploy_commit",
		"auto_deploy_commit":
		linkText = deployLinkText
		linkPath = deployPage
	}

	link, err := em.getInstanceLink(instanceData.Organization.ExternalID, linkPath)
	if err != nil {
		requestsError.With(prometheus.Labels{"status_code": http.StatusText(http.StatusBadRequest)}).Inc()
		return nil, http.StatusBadRequest, errors.Wrap(err, "cannot get Weave Cloud deploy page link")
	}

	e, err := buildEvent(body, sm, eventType, instanceID, instanceData.Organization.Name, notifLink, link, linkText)
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

	link, err := em.getInstanceLink(instanceData.Organization.ExternalID, notificationConfigPath)
	if err != nil {
		requestsError.With(prometheus.Labels{"status_code": http.StatusText(http.StatusBadRequest)}).Inc()
		return errors.Wrap(err, "cannot get Weave Cloud notification page link")
	}

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

		emailMsg, err := parser.EmailFromSlack(msg, eventType, instanceName, link)
		if err != nil {
			return errors.Wrap(err, "cannot get email message")
		}

		browserMsg, err := parser.BrowserFromSlack(types.SlackMessage{Text: msg}, eventType, "", "")
		if err != nil {
			return errors.Wrap(err, "cannot get email message")
		}

		msgJSON, err := json.Marshal(msg)
		if err != nil {
			return errors.Wrap(err, "cannot marshal message")
		}
		stackdriverMsg, err := parser.StackdriverFromSlack(msgJSON, eventType, instanceName)
		if err != nil {
			return errors.Wrap(err, "cannot get stackdriver message")
		}

		slackText := fmt.Sprintf("The address for *%s* was updated by %s!", receiver.RType, userEmail)
		slackMsg, err := parser.SlackFromSlack(types.SlackMessage{Text: slackText}, instanceName, link)
		if err != nil {
			return errors.Wrap(err, "cannot get slack text message")
		}

		event.Messages = map[string]json.RawMessage{
			types.EmailReceiver:       emailMsg,
			types.BrowserReceiver:     browserMsg,
			types.SlackReceiver:       slackMsg,
			types.StackdriverReceiver: stackdriverMsg,
		}
	} else if !reflect.DeepEqual(oldReceiver.EventTypes, receiver.EventTypes) {
		// eventTypes changed event

		visibleEvents, err := em.DB.ListEventTypes(nil, featureFlags)

		if err != nil {
			return err
		}

		added, removed := diff(oldReceiver.EventTypes, receiver.EventTypes, visibleEvents)
		if len(added) == 0 && len(removed) == 0 {
			// Nothing changed, no need to make noise
			return nil
		}

		text := formatEventTypeText("<b>", "</b>", receiver.RType, "<i>", "</i>", added, removed, userEmail)

		emailMsg, err := parser.EmailFromSlack(text, eventType, instanceName, link)
		if err != nil {
			return errors.Wrap(err, "cannot get email message")
		}

		browserMsg, err := parser.BrowserFromSlack(types.SlackMessage{Text: text}, eventType, "", "")
		if err != nil {
			return errors.Wrap(err, "cannot get email message")
		}

		textJSON, err := json.Marshal(text)
		if err != nil {
			return errors.Wrap(err, "cannot marshal message")
		}
		stackdriverMsg, err := parser.StackdriverFromSlack(textJSON, eventType, instanceName)
		if err != nil {
			return errors.Wrap(err, "cannot get stackdriver message for event types changed")
		}

		slackText := formatEventTypeText("*", "*", receiver.RType, "_", "_", added, removed, userEmail)
		slackMsg, err := parser.SlackFromSlack(types.SlackMessage{Text: slackText}, instanceName, link)
		if err != nil {
			return errors.Wrap(err, "cannot get slack message")
		}

		event.Messages = map[string]json.RawMessage{
			types.EmailReceiver:       emailMsg,
			types.BrowserReceiver:     browserMsg,
			types.SlackReceiver:       slackMsg,
			types.StackdriverReceiver: stackdriverMsg,
		}
	} else {
		// nothing changed, don't send event
		return nil
	}

	go func() {

		if _, err := em.storeAndSend(ctx, event, instanceData.Organization.FeatureFlags); err != nil {
			log.Warnf("failed to store in DB or send to SQS config change event: %s", err)
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

func buildEvent(body []byte, sm types.SlackMessage, etype, instanceID, instanceName, notificationPageLink, link, linkText string) (types.Event, error) {
	html := parser.SlackMsgToHTML(sm, instanceName, linkText, link)

	emailMsg, err := parser.EmailFromSlack(html, etype, instanceName, notificationPageLink)
	if err != nil {
		return types.Event{}, errors.Wrap(err, "cannot get email message")
	}

	browserMsg, err := parser.BrowserFromSlack(sm, etype, link, linkText)
	if err != nil {
		return types.Event{}, errors.Wrap(err, "cannot get email message")
	}

	stackdriverMsg, err := parser.StackdriverFromSlack(json.RawMessage(body), etype, instanceName)
	if err != nil {
		return types.Event{}, errors.Wrap(err, "cannot get stackdriver message")
	}

	// opsGenie message makes sense only for monitor event
	var opsGenieMsg json.RawMessage
	if etype == "monitor" {
		opsGenieMsg, err = parser.OpsGenieFromSlack(html, etype, instanceName)
		if err != nil {
			return types.Event{}, errors.Wrap(err, "cannot get OpsGenie message")
		}
	}

	slackMsg, err := parser.SlackFromSlack(sm, instanceName, link)
	if err != nil {
		return types.Event{}, errors.Wrap(err, "cannot get slack message")
	}

	var event types.Event
	event.InstanceID = instanceID
	event.Type = etype
	event.Timestamp = time.Now()
	event.Messages = map[string]json.RawMessage{
		types.BrowserReceiver:     browserMsg,
		types.SlackReceiver:       slackMsg,
		types.EmailReceiver:       emailMsg,
		types.StackdriverReceiver: stackdriverMsg,
		types.OpsGenieReceiver:    opsGenieMsg,
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

func (em *EventManager) getInstanceLink(externalID, resource string) (string, error) {
	url, err := url.Parse(em.WcURL)
	if err != nil {
		return "", errors.Wrapf(err, "cannot parse Weave Cloud URL %s", em.WcURL)
	}

	url.Path = path.Join(url.Path, externalID, resource)
	return url.String(), nil
}

func buildWebhookEvent(m types.WebhookAlert, etype, instanceID, instanceName, notificationPageLink, link, linkText string) (types.Event, error) {
	if len(m.Alerts) == 0 {
		return types.Event{}, errors.New("event is empty, alerts not found")
	}

	m.SettingsURL = notificationPageLink
	m.WeaveCloudURL = map[string]string{
		linkText: link,
	}

	emailMsg, err := parser.EmailFromAlert(m, etype, instanceName, notificationPageLink)
	if err != nil {
		return types.Event{}, errors.Wrap(err, "cannot get email message")
	}

	slackMsg, err := parser.SlackFromAlert(m, etype, instanceName, notificationPageLink)
	if err != nil {
		return types.Event{}, errors.Wrap(err, "cannot get slack message")
	}

	browserMsg, err := parser.BrowserFromAlert(m, etype)
	if err != nil {
		return types.Event{}, errors.Wrap(err, "cannot get browser message")
	}

	stackdriverMsg, err := parser.StackdriverFromAlert(m, etype, instanceName)
	if err != nil {
		return types.Event{}, errors.Wrap(err, "cannot get stackdriver message")
	}

	// opsGenie message makes sense only for monitor event
	var opsGenieMsg json.RawMessage
	if etype == "monitor" {
		opsGenieMsg, err = parser.OpsGenieFromAlert(m, etype, instanceName)
		if err != nil {
			return types.Event{}, errors.Wrap(err, "cannot get OpsGenie message")
		}
	}

	ev := types.Event{
		Type:         etype,
		InstanceID:   instanceID,
		InstanceName: instanceName,
		Timestamp:    time.Now(),
		Messages: map[string]json.RawMessage{
			types.BrowserReceiver:     browserMsg,
			types.SlackReceiver:       slackMsg,
			types.EmailReceiver:       emailMsg,
			types.StackdriverReceiver: stackdriverMsg,
			types.OpsGenieReceiver:    opsGenieMsg,
		},
	}

	return ev, nil
}
