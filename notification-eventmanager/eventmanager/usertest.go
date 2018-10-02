package eventmanager

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/weaveworks/service/notification-eventmanager/eventmanager/render"
	"github.com/weaveworks/service/notification-eventmanager/types"
	"github.com/weaveworks/service/users"
)

const userTestTitle = "Weave Cloud test event"

// UserTestData is data for user_test event
type UserTestData struct {
	UserEmail string `json:"user_email,omitempty"`
}

// handleTestEvent posts a test event for the user to verify things are working.
func (em *EventManager) handleTestEvent(r *http.Request, instanceID string) (interface{}, int, error) {
	timestamp := time.Now()
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
	etype := types.UserTestType

	userEmail, status, err := em.extractUserEmail(r)
	if err != nil {
		requestsError.With(prometheus.Labels{"status_code": http.StatusText(status)}).Inc()
		return nil, status, errors.Wrap(err, "error extracting user email for test event")
	}

	link, err := em.getInstanceLink(instanceData.Organization.ExternalID, notificationConfigPath)
	if err != nil {
		requestsError.With(prometheus.Labels{"status_code": http.StatusText(http.StatusInternalServerError)}).Inc()
		return nil, http.StatusInternalServerError, errors.Wrap(err, "error getting notification config page link for test event")
	}

	text := fmt.Sprintf("A test event triggered from Weave Cloud by %s!", userEmail)

	htmlText := fmt.Sprintf(`A test event triggered from <a href="%s">Weave Cloud</a> by %s!`, link, userEmail)
	htmlTextJSON, err := json.Marshal(htmlText)
	if err != nil {
		return nil, http.StatusInternalServerError, errors.Wrapf(err, "cannot marshal text: %s", htmlText)
	}

	sdMsg, err := render.StackdriverFromSlack(htmlTextJSON, etype, instanceName)
	if err != nil {
		requestsError.With(prometheus.Labels{"status_code": http.StatusText(http.StatusInternalServerError)}).Inc()
		return nil, http.StatusInternalServerError, errors.Wrap(err, "error getting stackdriver message for test event")
	}

	emailMsg, err := em.Render.EmailFromSlack(userTestTitle, htmlText, etype, instanceName, "", "", "", link, timestamp)
	if err != nil {
		requestsError.With(prometheus.Labels{"status_code": http.StatusText(http.StatusInternalServerError)}).Inc()
		return nil, http.StatusInternalServerError, errors.Wrap(err, "error getting email message for test event")
	}

	slackMsg, err := render.SlackFromSlack(types.SlackMessage{Text: text}, instanceName, link)
	if err != nil {
		requestsError.With(prometheus.Labels{"status_code": http.StatusText(http.StatusInternalServerError)}).Inc()
		return nil, http.StatusInternalServerError, errors.Wrap(err, "error getting slack text message for test event")
	}

	browserMsg, err := render.BrowserFromSlack(types.SlackMessage{Text: text}, etype, link, "Weave Cloud notification")
	if err != nil {
		requestsError.With(prometheus.Labels{"status_code": http.StatusText(http.StatusInternalServerError)}).Inc()
		return nil, http.StatusInternalServerError, errors.Wrap(err, "error getting browser message for test event")
	}

	opsGenieMsg, err := render.OpsGenieFromSlack(htmlText, etype, instanceName)
	if err != nil {
		requestsError.With(prometheus.Labels{"status_code": http.StatusText(http.StatusInternalServerError)}).Inc()
		return nil, http.StatusInternalServerError, errors.Wrap(err, "error getting OpsGenie message for test event")
	}

	pagerDutyMsg, err := render.PagerDutyFromSlack(text, etype, instanceName, link, "Weave Cloud notification")
	if err != nil {
		requestsError.With(prometheus.Labels{"status_code": http.StatusText(http.StatusInternalServerError)}).Inc()
		return nil, http.StatusInternalServerError, errors.Wrap(err, "error getting PagerDuty message for test event")
	}

	data := UserTestData{UserEmail: userEmail}

	dataBytes, err := json.Marshal(data)
	if err != nil {
		requestsError.With(prometheus.Labels{"status_code": http.StatusText(http.StatusInternalServerError)}).Inc()
		return nil, http.StatusInternalServerError, errors.Wrap(err, "error marshaling test event data")
	}

	testEvent := types.Event{
		Type:         etype,
		InstanceID:   instanceID,
		InstanceName: instanceData.Organization.Name,
		Timestamp:    timestamp,
		Data:         dataBytes,
		Messages: map[string]json.RawMessage{
			types.EmailReceiver:       emailMsg,
			types.BrowserReceiver:     browserMsg,
			types.SlackReceiver:       slackMsg,
			types.StackdriverReceiver: sdMsg,
			types.OpsGenieReceiver:    opsGenieMsg,
			types.PagerDutyReceiver:   pagerDutyMsg,
		},
	}

	eventID, err := em.storeAndSend(r.Context(), testEvent, instanceData.Organization.FeatureFlags)

	if err != nil {
		requestsError.With(prometheus.Labels{"status_code": http.StatusText(http.StatusInternalServerError)}).Inc()
		return nil, http.StatusInternalServerError, errors.Wrap(err, "cannot post and send test event")
	}

	return eventID, http.StatusOK, nil
}
