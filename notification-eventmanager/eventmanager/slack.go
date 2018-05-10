package eventmanager

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"

	"github.com/weaveworks/blackfriday"
	"github.com/weaveworks/service/notification-eventmanager/types"
	"github.com/weaveworks/service/users"
)

const (
	markdownNewline      = "  \n"
	markdownNewParagraph = "\n\n"
)

// handleSlackEvent handles slack json payload that includes the message text and some options, creates event, log it in DB and queue
func (em *EventManager) handleSlackEvent(w http.ResponseWriter, r *http.Request) {
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

	eventID, err := em.storeAndSend(r.Context(), e, instanceData.Organization.FeatureFlags)

	if err != nil {
		log.Errorf("cannot post and send %s event, error: %s", e.Type, err)
		http.Error(w, "Failed handle event", http.StatusInternalServerError)
		requestsError.With(prometheus.Labels{"status_code": http.StatusText(http.StatusInternalServerError)}).Inc()
		return
	}

	w.Write([]byte(eventID))
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
