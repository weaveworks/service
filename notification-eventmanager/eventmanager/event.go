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
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"

	"github.com/weaveworks/common/user"
	"github.com/weaveworks/service/notification-eventmanager/eventmanager/render"
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

	notifLink, err := em.getInstanceLink(instanceData.Organization.ExternalID, notificationConfigPath)
	if err != nil {
		requestsError.With(prometheus.Labels{"status_code": http.StatusText(http.StatusBadRequest)}).Inc()
		return nil, http.StatusBadRequest, errors.Wrap(err, "cannot get Weave Cloud notification page link")
	}

	var eventURL, eventURLText string
	// and link to Deploy page for Flux events
	switch e.Type {
	case types.SyncType,
		types.PolicyType,
		types.DeployType,
		types.AutoDeployType,
		types.DeployCommitType,
		types.AutoDeployCommitType:
		eventURLText = deployLinkText
		eventURL = deployPage
	}

	link, err := em.getInstanceLink(instanceData.Organization.ExternalID, eventURL)
	if err != nil {
		requestsError.With(prometheus.Labels{"status_code": http.StatusText(http.StatusBadRequest)}).Inc()
		return nil, http.StatusBadRequest, errors.Wrap(err, "cannot get Weave Cloud page link")
	}

	if e.Data != nil {
		if err := em.Render.Data(&e, link, eventURLText, notifLink); err != nil {
			requestsError.With(prometheus.Labels{"status_code": http.StatusText(http.StatusInternalServerError)}).Inc()
			return nil, http.StatusInternalServerError, errors.Wrap(err, "unable to parse event data")
		}
	}

	eventID, err := em.storeAndSend(r.Context(), e, instanceData.Organization.FeatureFlags)

	if err != nil {
		requestsError.With(prometheus.Labels{"status_code": http.StatusText(http.StatusInternalServerError)}).Inc()
		return nil, http.StatusInternalServerError, errors.Wrap(err, "failed to handle event")
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
	case types.MonitorType:
		linkText = alertLinkText
		linkPath = alertsPage
	case types.SyncType,
		types.PolicyType,
		types.DeployType,
		types.AutoDeployType,
		types.DeployCommitType,
		types.AutoDeployCommitType:
		linkText = deployLinkText
		linkPath = deployPage
	}

	link, err := em.getInstanceLink(instanceData.Organization.ExternalID, linkPath)
	if err != nil {
		requestsError.With(prometheus.Labels{"status_code": http.StatusText(http.StatusBadRequest)}).Inc()
		return nil, http.StatusBadRequest, errors.Wrap(err, "cannot get Weave Cloud deploy page link")
	}

	e, err := em.buildEvent(body, sm, eventType, instanceID, instanceData.Organization.Name, notifLink, link, linkText)
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
		return nil, 0, err
	}

	return events, http.StatusOK, nil
}

func (em *EventManager) buildEvent(body []byte, sm types.SlackMessage, etype, instanceID, instanceName, notificationPageLink, link, linkText string) (types.Event, error) {
	html := render.SlackMsgToHTML(sm, instanceName, linkText, link)
	timestamp := time.Now()
	emailMsg, err := em.Render.EmailFromSlack(etype, html, etype, instanceName, link, linkText, notificationPageLink, timestamp)
	if err != nil {
		return types.Event{}, errors.Wrap(err, "cannot get email message")
	}

	browserMsg, err := render.BrowserFromSlack(sm, etype, link, linkText)
	if err != nil {
		return types.Event{}, errors.Wrap(err, "cannot get email message")
	}

	stackdriverMsg, err := render.StackdriverFromSlack(json.RawMessage(body), etype, instanceName)
	if err != nil {
		return types.Event{}, errors.Wrap(err, "cannot get stackdriver message")
	}

	// opsGenie message makes sense only for monitor event
	var opsGenieMsg json.RawMessage
	if etype == types.MonitorType {
		opsGenieMsg, err = render.OpsGenieFromSlack(html, etype, instanceName)
		if err != nil {
			return types.Event{}, errors.Wrap(err, "cannot get OpsGenie message")
		}
	}

	slackMsg, err := render.SlackFromSlack(sm, instanceName, link)
	if err != nil {
		return types.Event{}, errors.Wrap(err, "cannot get slack message")
	}

	var event types.Event
	event.InstanceID = instanceID
	event.Type = etype
	event.Timestamp = timestamp
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
