package eventmanager

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"github.com/weaveworks/service/notification-eventmanager/types"
	"github.com/weaveworks/service/users"
)

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

	var wa types.WebhookAlert
	if err = json.Unmarshal(body, &wa); err != nil {
		requestsError.With(prometheus.Labels{"status_code": http.StatusText(http.StatusBadRequest)}).Inc()
		return nil, http.StatusBadRequest, errors.Wrap(err, "cannot unmarshal body")
	}

	notifLink, err := em.getInstanceLink(instanceData.Organization.ExternalID, notificationConfigPath)
	if err != nil {
		requestsError.With(prometheus.Labels{"status_code": http.StatusText(http.StatusBadRequest)}).Inc()
		return nil, http.StatusBadRequest, errors.Wrap(err, "cannot get Weave Cloud notification page link")
	}

	alertsConfigLink, err := em.getInstanceLink(instanceData.Organization.ExternalID, alertsConfigPath)
	if err != nil {
		requestsError.With(prometheus.Labels{"status_code": http.StatusText(http.StatusBadRequest)}).Inc()
		return nil, http.StatusBadRequest, errors.Wrap(err, "cannot get Weave Cloud alert config page link")
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

	e, err := em.Render.BuildCortexEvent(wa, eventType, instanceID, instanceData.Organization.Name, notifLink, alertsConfigLink, link, linkText)
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
