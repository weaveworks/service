package eventmanager

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"github.com/weaveworks/service/notification-eventmanager/eventmanager/render"
	"github.com/weaveworks/service/notification-eventmanager/types"
	"github.com/weaveworks/service/users"
)

// MonitorData is data for monitor event
type MonitorData struct {
	GroupKey          string            `json:"groupKey,omitempty"`
	Status            string            `json:"status,omitempty"`
	Receiver          string            `json:"receiver,omitempty"`
	GroupLabels       map[string]string `json:"groupLabels,omitempty"`
	CommonLabels      map[string]string `json:"commonLabels,omitempty"`
	CommonAnnotations map[string]string `json:"commonAnnotations,omitempty"`
	Alerts            []types.Alert     `json:"alerts,omitempty"`
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
	case types.MonitorType:
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

func buildWebhookEvent(m types.WebhookAlert, etype, instanceID, instanceName, notificationPageLink, link, linkText string) (types.Event, error) {
	if len(m.Alerts) == 0 {
		return types.Event{}, errors.New("event is empty, alerts not found")
	}

	m.SettingsURL = notificationPageLink
	m.WeaveCloudURL = map[string]string{
		linkText: link,
	}

	emailMsg, err := render.EmailFromAlert(m, etype, instanceName, notificationPageLink)
	if err != nil {
		return types.Event{}, errors.Wrap(err, "cannot get email message")
	}

	slackMsg, err := render.SlackFromAlert(m, etype, instanceName, notificationPageLink)
	if err != nil {
		return types.Event{}, errors.Wrap(err, "cannot get slack message")
	}

	browserMsg, err := render.BrowserFromAlert(m, etype)
	if err != nil {
		return types.Event{}, errors.Wrap(err, "cannot get browser message")
	}

	stackdriverMsg, err := render.StackdriverFromAlert(m, etype, instanceName)
	if err != nil {
		return types.Event{}, errors.Wrap(err, "cannot get stackdriver message")
	}

	// opsGenie message makes sense only for monitor event
	var opsGenieMsg json.RawMessage
	if etype == types.MonitorType {
		opsGenieMsg, err = render.OpsGenieFromAlert(m, etype, instanceName)
		if err != nil {
			return types.Event{}, errors.Wrap(err, "cannot get OpsGenie message")
		}
	}

	data, err := json.Marshal(MonitorData{
		GroupKey:          m.GroupKey,
		Status:            m.Status,
		Receiver:          m.Receiver,
		GroupLabels:       m.GroupLabels,
		CommonLabels:      m.CommonLabels,
		CommonAnnotations: m.CommonAnnotations,
		Alerts:            m.Alerts,
	})
	if err != nil {
		return types.Event{}, errors.Wrap(err, "error marshaling monitor event data")
	}

	ev := types.Event{
		Type:         etype,
		InstanceID:   instanceID,
		InstanceName: instanceName,
		Timestamp:    time.Now(),
		Data:         data,
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
