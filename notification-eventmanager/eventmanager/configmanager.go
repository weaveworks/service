package eventmanager

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"time"

	"github.com/badoux/checkmail" // Import the postgres sql driver
	"github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"
	log "github.com/sirupsen/logrus"
	"github.com/weaveworks/service/notification-eventmanager/types"
	serviceUsers "github.com/weaveworks/service/users"
	_ "gopkg.in/mattes/migrate.v1/driver/postgres" // Import the postgres migrations driver
)

const (
	// MaxEventsList is the highest number of events that can be requested in one list call
	MaxEventsList = 10000
)

var (
	// isWebHookPath regexp checks string contains only letters, numbers and slashes
	// for url.Path in slack webhook (services/T00000000/B00000000/XXXXXXXXXXXXXXXXXXXXXXXX)
	isWebHookPath = regexp.MustCompile(`^[A-Za-z0-9\/]+$`).MatchString
)

func (em *EventManager) listEventTypes(r *http.Request) (interface{}, int, error) {
	result, err := em.DB.ListEventTypes(nil, getFeatureFlags(r))
	if err != nil {
		return nil, 0, err
	}
	return result, http.StatusOK, nil
}

func (em *EventManager) listReceivers(r *http.Request, instanceID string) (interface{}, int, error) {
	result, err := em.DB.ListReceivers(instanceID)
	if err != nil {
		return nil, 0, err
	}
	return result, http.StatusOK, nil
}

func (em *EventManager) createReceiver(r *http.Request, instanceID string) (interface{}, int, error) {
	receiver := types.Receiver{}
	err := parseBody(r, &receiver)
	if err != nil {
		log.Errorf("cannot parse body, error: %s", err)
		return "Bad request body", http.StatusBadRequest, nil
	}
	if err := isValidAddress(receiver.AddressData, receiver.RType); err != nil {
		log.Errorf("address validation failed for %s address, error: %s", receiver.RType, err)
		return "address validation failed", http.StatusBadRequest, nil
	}
	// Validate they only set the fields we're going to use
	if receiver.ID != "" || receiver.InstanceID != "" || len(receiver.EventTypes) != 0 {
		return errors.New("ID, instance and event types should not be specified"), http.StatusBadRequest, nil
	}
	result, err := em.DB.CreateReceiver(receiver, instanceID)
	if err != nil {
		return nil, 0, err
	}
	return result, http.StatusOK, nil
}

// isValidAddress checks if address is valid for receiver type
func isValidAddress(addressData json.RawMessage, rtype string) error {
	switch rtype {
	case types.SlackReceiver:
		var addrStr string
		if err := json.Unmarshal(addressData, &addrStr); err != nil {
			return errors.Wrapf(err, "cannot unmarshal %s receiver address data %s", rtype, addressData)
		}

		url, err := url.ParseRequestURI(addrStr)
		if err != nil {
			return errors.Wrapf(err, "cannot parse URI %s", addrStr)
		}
		if url.Scheme != "https" || url.Port() != "" || url.Host != "hooks.slack.com" || !isWebHookPath(url.Path) {
			return errors.Errorf("invalid slack webhook URL %s", addrStr)
		}

	case types.EmailReceiver:
		var addrStr string
		if err := json.Unmarshal(addressData, &addrStr); err != nil {
			return errors.Wrapf(err, "cannot unmarshal %s receiver address data %s", rtype, addressData)
		}

		if err := checkmail.ValidateFormat(addrStr); err != nil {
			return errors.Wrapf(err, "invalid email address %s", addrStr)
		}

	case types.BrowserReceiver:
		return nil

	case types.StackdriverReceiver:
		fields := []string{
			"type",
			"project_id",
			"private_key_id",
			"private_key",
			"client_email",
			"client_id",
			"auth_uri",
			"token_uri",
			"auth_provider_x509_cert_url",
			"client_x509_cert_url",
		}
		var creds map[string]string
		if err := json.Unmarshal(addressData, &creds); err != nil {
			return errors.Wrapf(err, "cannot unmarshal %s receiver address data", rtype)
		}

		for _, v := range fields {
			if creds[v] == "" || (v == "type" && creds[v] != "service_account") {
				return errors.Errorf("invalid stackdriver receiver")
			}
		}
	}

	return nil
}

func (em *EventManager) getReceiver(r *http.Request, instanceID string, receiverID string) (interface{}, int, error) {
	if _, err := uuid.FromString(receiverID); err != nil {
		// Bad identifier
		return nil, http.StatusNotFound, nil
	}
	featureFlags := getFeatureFlags(r)
	result, err := em.DB.GetReceiver(instanceID, receiverID, featureFlags)
	if err != nil {
		return nil, 0, err
	}
	return result, http.StatusOK, nil
}

// CreateConfigChangedEvent creates event with changed configuration
func (em *EventManager) createConfigChangedEvent(ctx context.Context, instanceID string, oldReceiver, receiver types.Receiver, eventTime time.Time) error {
	log.Debug("update_config Event Firing...")

	eventType := "config_changed"
	event := types.Event{
		Type:       eventType,
		InstanceID: instanceID,
		Timestamp:  eventTime,
	}

	instanceData, err := em.UsersClient.GetOrganization(ctx, &serviceUsers.GetOrganizationRequest{
		ID: &serviceUsers.GetOrganizationRequest_InternalID{InternalID: instanceID},
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

		event.Messages = map[string]json.RawMessage{
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
		if err := em.storeAndSend(ctx, event, instanceData.Organization.FeatureFlags); err != nil {
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

func (em *EventManager) updateReceiver(r *http.Request, instanceID string, receiverID string) (interface{}, int, error) {
	eventTime := time.Now()

	if _, err := uuid.FromString(receiverID); err != nil {
		// Bad identifier
		return nil, http.StatusNotFound, nil
	}

	receiver := types.Receiver{}
	err := parseBody(r, &receiver)
	if err != nil {
		return "Bad request body", http.StatusBadRequest, nil
	}
	if (receiver.ID != "" && receiver.ID != receiverID) || (receiver.ID != "" && receiver.InstanceID != instanceID) {
		return "Receiver ID and instance ID cannot be modified", http.StatusBadRequest, nil
	}

	if err := isValidAddress(receiver.AddressData, receiver.RType); err != nil {
		log.Errorf("address validation failed for %s address, error: %s", receiver.RType, err)
		return "address validation failed", http.StatusBadRequest, nil
	}

	if receiver.ID == "" {
		receiver.ID = receiverID
	}
	featureFlags := getFeatureFlags(r)

	// before transaction changes the addressData and eventTypes, get oldReceiver which has oldAddressData and oldEventTypes
	oldReceiver, err := em.DB.GetReceiver(instanceID, receiverID, featureFlags)
	if err == sql.ErrNoRows {
		return nil, http.StatusNotFound, nil
	}
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	if oldReceiver.RType != receiver.RType {
		return nil, 0, errors.New("Receiver type cannot be modified")
	}

	// Transaction to actually update the receiver in DB
	if err := em.DB.UpdateReceiver(receiver, instanceID, featureFlags); err != nil {
		if err == sql.ErrNoRows {
			return nil, http.StatusNotFound, nil
		}
		return nil, 0, err
	}

	// all good!
	// Fire event every time config is successfully changed
	go func() {
		eventErr := em.createConfigChangedEvent(context.Background(), instanceID, oldReceiver, receiver, eventTime)
		if eventErr != nil {
			log.Error(eventErr)
		}
	}()

	return nil, http.StatusOK, nil
}

func (em *EventManager) deleteReceiver(r *http.Request, instanceID string, receiverID string) (interface{}, int, error) {
	if _, err := uuid.FromString(receiverID); err != nil {
		// Bad identifier
		return nil, http.StatusNotFound, nil
	}
	affected, err := em.DB.DeleteReceiver(instanceID, receiverID)
	if err != nil {
		return nil, 0, err
	}
	if affected == 0 {
		return nil, http.StatusNotFound, nil
	}
	return nil, http.StatusOK, nil
}

func (em *EventManager) getEvents(r *http.Request, instanceID string) (interface{}, int, error) {
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
		var fieldsMap map[string]struct{}
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
