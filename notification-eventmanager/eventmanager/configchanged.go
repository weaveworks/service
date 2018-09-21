package eventmanager

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"time"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"github.com/weaveworks/service/notification-eventmanager/eventmanager/render"
	"github.com/weaveworks/service/notification-eventmanager/types"
	"github.com/weaveworks/service/users"
)

const configChangeTitle = "Weave Cloud notification config changed"

// ConfigChangedData is data for config_changed event
type ConfigChangedData struct {
	UserEmail    string   `json:"user_email,omitempty"`
	ReceiverType string   `json:"receiver_type,omitempty"`
	Enabled      []string `json:"enabled,omitempty"`
	Disabled     []string `json:"disabled,omitempty"`
}

// createConfigChangedEvent creates event with changed configuration
func (em *EventManager) createConfigChangedEvent(ctx context.Context, instanceID string, oldReceiver, receiver types.Receiver, eventTime time.Time, userEmail string, featureFlags []string) error {
	log.Debug("update_config Event Firing...")

	eventType := types.ConfigChangedType

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

	ev := types.Event{
		Type:         eventType,
		InstanceID:   instanceID,
		InstanceName: instanceName,
		Timestamp:    eventTime,
	}

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

		emailMsg, err := em.Render.EmailFromSlack(configChangeTitle, msg, eventType, instanceName, "", "", "", link, eventTime)
		if err != nil {
			return errors.Wrap(err, "cannot get email message")
		}

		browserMsg, err := render.BrowserFromSlack(types.SlackMessage{Text: msg}, eventType, link, "Weave Cloud notification")
		if err != nil {
			return errors.Wrap(err, "cannot get email message")
		}

		msgJSON, err := json.Marshal(msg)
		if err != nil {
			return errors.Wrap(err, "cannot marshal message")
		}
		stackdriverMsg, err := render.StackdriverFromSlack(msgJSON, eventType, instanceName)
		if err != nil {
			return errors.Wrap(err, "cannot get stackdriver message")
		}

		slackText := fmt.Sprintf("The address for *%s* was updated by %s!", receiver.RType, userEmail)
		slackMsg, err := render.SlackFromSlack(types.SlackMessage{Text: slackText}, instanceName, link)
		if err != nil {
			return errors.Wrap(err, "cannot get slack text message")
		}

		data, err := json.Marshal(ConfigChangedData{
			UserEmail:    userEmail,
			ReceiverType: receiver.RType,
		})
		if err != nil {
			return errors.Wrap(err, "cannot marshal config_change event data")
		}

		ev.Data = data

		ev.Messages = map[string]json.RawMessage{
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

		emailMsg, err := em.Render.EmailFromSlack(configChangeTitle, text, eventType, instanceName, "", "", "", link, eventTime)
		if err != nil {
			return errors.Wrap(err, "cannot get email message")
		}

		browserMsg, err := render.BrowserFromSlack(types.SlackMessage{Text: text}, eventType, link, "Weave Cloud notification")
		if err != nil {
			return errors.Wrap(err, "cannot get email message")
		}

		textJSON, err := json.Marshal(text)
		if err != nil {
			return errors.Wrap(err, "cannot marshal message")
		}
		stackdriverMsg, err := render.StackdriverFromSlack(textJSON, eventType, instanceName)
		if err != nil {
			return errors.Wrap(err, "cannot get stackdriver message for event types changed")
		}

		slackText := formatEventTypeText("*", "*", receiver.RType, "_", "_", added, removed, userEmail)
		slackMsg, err := render.SlackFromSlack(types.SlackMessage{Text: slackText}, instanceName, link)
		if err != nil {
			return errors.Wrap(err, "cannot get slack message")
		}

		data, err := json.Marshal(ConfigChangedData{
			UserEmail:    userEmail,
			ReceiverType: receiver.RType,
			Enabled:      added,
			Disabled:     removed,
		})
		if err != nil {
			return errors.Wrap(err, "cannot marshal config_change event data")
		}

		ev.Data = data

		ev.Messages = map[string]json.RawMessage{
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

		if _, err := em.storeAndSend(ctx, ev, instanceData.Organization.FeatureFlags); err != nil {
			log.Warnf("failed to store in DB or send to SQS config change event: %s", err)
		}
	}()

	return nil
}
