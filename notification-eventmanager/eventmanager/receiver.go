package eventmanager

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/url"
	"regexp"
	"time"

	"github.com/badoux/checkmail"
	"github.com/pkg/errors"
	"github.com/satori/go.uuid"
	log "github.com/sirupsen/logrus"

	"github.com/weaveworks/service/notification-eventmanager/types"
)

const (
	Browser     = "browser"
	Email       = "email"
	Slack       = "slack"
	Stackdriver = "stackdriver"
)

var (
	// isWebHookPath regexp checks string contains only letters, numbers and slashes
	// for url.Path in slack webhook (services/T00000000/B00000000/XXXXXXXXXXXXXXXXXXXXXXXX)
	isWebHookPath = regexp.MustCompile(`^[A-Za-z0-9\/]+$`).MatchString
)

func (em *EventManager) handleListReceivers(r *http.Request, instanceID string) (interface{}, int, error) {
	result, err := em.DB.ListReceivers(instanceID)
	if err != nil {
		return nil, 0, err
	}
	return result, http.StatusOK, nil
}

func (em *EventManager) handleCreateReceiver(r *http.Request, instanceID string) (interface{}, int, error) {
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

func (em *EventManager) handleGetReceiver(r *http.Request, instanceID string, receiverID string) (interface{}, int, error) {
	if _, err := uuid.FromString(receiverID); err != nil {
		// Bad identifier
		return nil, http.StatusNotFound, nil
	}
	featureFlags := getFeatureFlags(r)
	result, err := em.DB.GetReceiver(instanceID, receiverID, featureFlags, false)
	if err == sql.ErrNoRows {
		return nil, http.StatusNotFound, nil
	}

	if err != nil {
		return nil, 0, err
	}
	return result, http.StatusOK, nil
}

func (em *EventManager) handleUpdateReceiver(r *http.Request, instanceID string, receiverID string) (interface{}, int, error) {
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
	oldReceiver, err := em.DB.GetReceiver(instanceID, receiverID, featureFlags, true)
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

func (em *EventManager) handleDeleteReceiver(r *http.Request, instanceID string, receiverID string) (interface{}, int, error) {
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