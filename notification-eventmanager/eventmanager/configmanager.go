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

	sq "github.com/Masterminds/squirrel"

	"github.com/badoux/checkmail"
	"github.com/lib/pq" // Import the postgres sql driver
	"github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"
	log "github.com/sirupsen/logrus"
	"github.com/weaveworks/service/notification-eventmanager/types"
	"github.com/weaveworks/service/notification-eventmanager/utils"
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

// for each row returned from query, calls callback
func (em *EventManager) forEachRow(rows *sql.Rows, callback func(*sql.Rows) error) error {
	var err error
	for err == nil && rows.Next() {
		err = callback(rows)
	}
	if err == nil {
		err = rows.Err()
	}
	return err
}

// Executes the given function in a transaction, and rolls back or commits depending on if the function errors.
// Ignores errors from rollback.
func (em *EventManager) withTx(method string, f func(tx *utils.Tx) error) error {
	tx, err := em.DB.Begin(method)
	if err != nil {
		return err
	}
	err = f(tx)
	if err == nil {
		return tx.Commit()
	}
	_ = tx.Rollback()

	if strings.Contains(err.Error(), "pq: duplicate key value violates unique constraint \"instances_initialized_pkey\"") {
		// if error is pq: duplicate key value violates unique constraint "instances_initialized_pkey"
		// instance already initialized, ignore this error
		return nil
	}

	return err
}

// Called before any handlers involving receivers, to initialize receiver defaults for the instance
// if it hasn't been already.
func (em *EventManager) checkInstanceDefaults(instanceID string) error {
	return em.withTx("check_instance_defaults_tx", func(tx *utils.Tx) error {
		// Test if instance is already initialized
		row := tx.QueryRow("check_instance_initialized", "SELECT 1 FROM instances_initialized WHERE instance_id = $1", instanceID)
		var unused int // We're only interested if a row is present, if it is the value will be 1.
		if err := row.Scan(&unused); err == nil {
			// No err means row was present, ie. instance already initialized. We're done.
			return nil
		} else if err != sql.ErrNoRows {
			// Actual DB error
			return err
		}
		// otherwise, err == ErrNoRows, ie. instance not initialized yet.

		// Hard-coded instance defaults, at least for now.
		receiver := types.Receiver{
			RType:       types.BrowserReceiver,
			AddressData: json.RawMessage("null"),
		}
		if _, _, err := em.createReceiver(tx, receiver, instanceID); err != nil {
			return err
		}

		// finally, insert instance_id into instances_initialized to mark it as done
		_, err := tx.Exec("set_instance_initialized", "INSERT INTO instances_initialized (instance_id) VALUES ($1)", instanceID)
		return err
	})
}

func (em *EventManager) httpListEventTypes(r *http.Request) (interface{}, int, error) {
	result, err := em.listEventTypes(nil, getFeatureFlags(r))
	if err != nil {
		return nil, 0, err
	}
	return result, http.StatusOK, nil
}

// Return a list of event types from the DB, either using given transaction or without a transaction if nil.
// Also filter by enabled feature flags if featureFlags is not nil.
func (em *EventManager) listEventTypes(tx *utils.Tx, featureFlags []string) ([]types.EventType, error) {
	queryFn := em.DB.Query
	if tx != nil {
		queryFn = tx.Query
	}
	// Exclude feature-flagged rows only if a) feature flags given is not nil and b) row has a feature flag
	// and c) feature flag isn't in given list of feature flags.
	rows, err := queryFn(
		"list_event_types",
		`SELECT name, display_name, description, default_receiver_types, feature_flag FROM event_types
		WHERE $1::text[] IS NULL OR feature_flag IS NULL OR feature_flag = ANY ($1::text[])`,
		pq.Array(featureFlags),
	)
	if err != nil {
		return nil, err
	}
	eventTypes := []types.EventType{}
	err = em.forEachRow(rows, func(row *sql.Rows) error {
		et, err := types.EventTypeFromRow(row)
		if err != nil {
			return err
		}
		eventTypes = append(eventTypes, et)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return eventTypes, nil
}

func (em *EventManager) listReceivers(r *http.Request, instanceID string) (interface{}, int, error) {
	if err := em.checkInstanceDefaults(instanceID); err != nil {
		return nil, 0, err
	}
	// In the below query, note the array_remove to transform [null] to [] if there are no matching rows.
	// Note we exclude event types with non-matching feature flags.
	rows, err := em.DB.Query(
		"list_receivers",
		`SELECT r.receiver_id, r.receiver_type, r.instance_id, r.address_data,
			array_remove(array_agg(rt.event_type), NULL)
		FROM receivers r
		LEFT JOIN receiver_event_types rt ON (r.receiver_id = rt.receiver_id)
		LEFT JOIN event_types et ON (rt.event_type = et.name)
		WHERE r.instance_id = $1
		GROUP BY r.receiver_id`,
		instanceID,
	)
	if err != nil {
		return nil, 0, err
	}
	receivers := []types.Receiver{}
	err = em.forEachRow(rows, func(row *sql.Rows) error {
		r, err := types.ReceiverFromRow(row)
		if err != nil {
			return err
		}
		receivers = append(receivers, r)
		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	return receivers, http.StatusOK, err
}

func (em *EventManager) httpCreateReceiver(r *http.Request, instanceID string) (interface{}, int, error) {
	if err := em.checkInstanceDefaults(instanceID); err != nil {
		return nil, 0, err
	}
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
	var result interface{}
	var code int
	err = em.withTx("create_receiver_tx", func(tx *utils.Tx) error {
		var err error
		result, code, err = em.createReceiver(tx, receiver, instanceID)
		return err
	})
	if err != nil {
		return nil, 0, err
	}
	return result, code, nil
}

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

func (em *EventManager) createReceiver(tx *utils.Tx, receiver types.Receiver, instanceID string) (interface{}, int, error) {
	// Validate they only set the fields we're going to use
	if receiver.ID != "" || receiver.InstanceID != "" || len(receiver.EventTypes) != 0 {
		return "ID, instance and event types should not be specified", http.StatusBadRequest, nil
	}
	// Re-encode the address data because the sql driver doesn't understand json columns
	// TODO validate this field against the specific receiver type
	encodedAddress, err := json.Marshal(receiver.AddressData)
	if err != nil {
		return nil, 0, err // This is a server error, because this round-trip *should* work.
	}
	// In a single transaction to prevent races (receiver gets edited before we're finished setting default event types),
	// we create the new Receiver row, look up what event types it should default to handling, and add those.
	row := tx.QueryRow(
		"create_receiver",
		`INSERT INTO receivers (instance_id, receiver_type, address_data)
		VALUES ($1, $2, $3)
		RETURNING receiver_id`,
		instanceID,
		receiver.RType,
		encodedAddress,
	)
	var receiverID string
	err = row.Scan(&receiverID)
	if err != nil {
		return nil, 0, err
	}
	_, err = tx.Exec(
		"set_receiver_defaults",
		`INSERT INTO receiver_event_types (receiver_id, event_type)
			SELECT $1, name
			FROM event_types
			WHERE $2 = ANY (default_receiver_types)`,
		receiverID,
		receiver.RType,
	)
	if err != nil {
		return nil, 0, err
	}
	return receiverID, http.StatusCreated, nil
}

func (em *EventManager) getReceiver(r *http.Request, instanceID string, receiverID string) (interface{}, int, error) {
	if err := em.checkInstanceDefaults(instanceID); err != nil {
		return nil, 0, err
	}
	if _, err := uuid.FromString(receiverID); err != nil {
		// Bad identifier
		return nil, http.StatusNotFound, nil
	}
	// In the below query, note the array_remove to transform [null] to [] if there are no matching rows.
	row := em.DB.QueryRow(
		"get_receiver",
		`SELECT r.receiver_id, r.receiver_type, r.instance_id, r.address_data,
			array_remove(array_agg(rt.event_type), NULL)
		FROM receivers r
		LEFT JOIN receiver_event_types rt ON (r.receiver_id = rt.receiver_id)
		LEFT JOIN event_types et ON (rt.event_type = et.name)
		WHERE r.receiver_id = $1 AND r.instance_id = $2 AND (et.feature_flag IS NULL OR et.feature_flag = ANY ($3))
		GROUP BY r.receiver_id`,
		receiverID,
		instanceID,
		pq.Array(getFeatureFlags(r)),
	)
	receiver, err := types.ReceiverFromRow(row)
	if err == sql.ErrNoRows {
		return nil, http.StatusNotFound, nil
	}
	if err != nil {
		return nil, 0, err
	}
	return receiver, http.StatusOK, nil
}

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
		if err := em.storeAndSend(ctx, event); err != nil {
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
	if err := em.checkInstanceDefaults(instanceID); err != nil {
		return nil, 0, err
	}
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

	// Re-encode the address data because the sql driver doesn't understand json columns
	// TODO validate this field against the specific receiver type
	encodedAddress, err := json.Marshal(receiver.AddressData)
	if err != nil {
		return nil, 0, err // This is a server error, because this round-trip *should* work.
	}

	if receiver.ID == "" {
		receiver.ID = receiverID
	}

	// before transaction changes the addressData and eventTypes, get oldReceiver which has oldAddressData and oldEventTypes
	receiverOrNil, status, err := em.getReceiver(r, instanceID, receiverID)
	gotOldReceiverValues := true
	if err != nil {
		gotOldReceiverValues = false
		log.Errorf("error with c.getReceiver call. Status: %v\nError: %s", status, err)
	}

	oldReceiver, ok := receiverOrNil.(types.Receiver)
	if !ok {
		gotOldReceiverValues = false
		log.Errorf("error converting result of getReceiver call, %v to types.Receiver", receiverOrNil)
	}

	// Transaction to actually update the receiver in DB
	userErrorMsg, code, err := em.updateReceiverTX(r, receiver, instanceID, encodedAddress)
	if err != nil {
		return nil, 0, err
	}
	if code == http.StatusOK {
		// all good!
		// Fire event every time config is successfully changed
		if gotOldReceiverValues {
			go func() {
				eventErr := em.createConfigChangedEvent(context.Background(), instanceID, oldReceiver, receiver, eventTime)
				if eventErr != nil {
					log.Error(eventErr)
				}
			}()
		} else {
			log.Error("Event config_changed not sent. Old receiver values not acquired.")
		}
		return nil, code, nil
	}
	return userErrorMsg, code, nil
}

func (em *EventManager) updateReceiverTX(r *http.Request, receiver types.Receiver, instanceID string, encodedAddress []byte) (string, int, error) {
	// We need to read some DB values to validate the new data (we could just rely on the DB's constraints, but it's hard
	// to return a meaningful error message if we do that). We do this in a transaction to prevent races.
	// We set these vars as a way of returning more values from inside the closure, since the function can only return error.
	code := http.StatusOK
	userErrorMsg := ""
	err := em.withTx("update_receiver_tx", func(tx *utils.Tx) error {
		// Verify receiver exists, has correct instance id and type.
		var rtype string
		err := tx.QueryRow("check_receiver_exists", `SELECT receiver_type FROM receivers WHERE receiver_id = $1 AND instance_id = $2`, receiver.ID, instanceID).
			Scan(&rtype)
		if err == sql.ErrNoRows {
			code = http.StatusNotFound
			return nil
		}
		if err != nil {
			return err
		}
		if rtype != receiver.RType {
			userErrorMsg = "Receiver type cannot be modified"
			code = http.StatusBadRequest
			return nil
		}
		// Verify event types list is valid by querying for items in the input but not in event_types
		rows, err := tx.Query(
			"check_new_receiver_event_types",
			`SELECT unnest FROM unnest($1::text[])
			WHERE unnest NOT IN (SELECT name FROM event_types)`,
			pq.Array(receiver.EventTypes),
		)
		if err != nil {
			return err
		}
		badTypes := []string{}
		for rows.Next() {
			var badType string
			err = rows.Scan(&badType)
			if err != nil {
				return err
			}
			badTypes = append(badTypes, badType)
		}
		err = rows.Err()
		if err != nil {
			return err
		}
		if len(badTypes) != 0 {
			userErrorMsg = fmt.Sprintf("Given event types do not exist: %s", strings.Join(badTypes, ", "))
			code = http.StatusNotFound
			return nil
		}
		// Verified all good, do the update.
		// First, update the actual record
		_, err = tx.Exec(
			"update_receiver",
			`UPDATE receivers SET (address_data) = ($2)
			WHERE receiver_id = $1`,
			receiver.ID,
			encodedAddress,
		)
		if err != nil {
			return err
		}
		// Delete any newly-dropped event types. Note we keep feature-flag-hidden event types,
		// since the client wouldn't have known about these so omitting them was not an intentional delete.
		_, err = tx.Exec(
			"remove_receiver_event_types",
			`DELETE FROM receiver_event_types
			WHERE receiver_id = $1 AND NOT event_type IN (
				SELECT name
				FROM event_types
				WHERE name = ANY ($2) OR NOT (feature_flag IS NULL OR feature_flag = ANY ($3))
			)`,
			receiver.ID,
			pq.Array(receiver.EventTypes),
			pq.Array(getFeatureFlags(r)),
		)
		if err != nil {
			return err
		}
		// Add any new event types
		_, err = tx.Exec(
			"add_receiver_event_types",
			`INSERT INTO receiver_event_types (receiver_id, event_type) (
				SELECT $1, unnest FROM unnest($2::text[])
			) ON CONFLICT DO NOTHING`,
			receiver.ID,
			pq.Array(receiver.EventTypes),
		)
		return err
	})

	return userErrorMsg, code, err
}

func (em *EventManager) deleteReceiver(r *http.Request, instanceID string, receiverID string) (interface{}, int, error) {
	if err := em.checkInstanceDefaults(instanceID); err != nil {
		return nil, 0, err
	}
	if _, err := uuid.FromString(receiverID); err != nil {
		// Bad identifier
		return nil, http.StatusNotFound, nil
	}
	result, err := em.DB.Exec(
		"delete_receiver",
		`DELETE FROM receivers
		WHERE receiver_id = $1 AND instance_id = $2`,
		receiverID,
		instanceID,
	)
	if err != nil {
		return nil, 0, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, 0, err
	}
	if affected == 0 {
		return nil, http.StatusNotFound, nil
	}
	return nil, http.StatusOK, nil
}

// GetReceiversForEvent returns all receivers for event from DB
func (em *EventManager) GetReceiversForEvent(event types.Event) ([]types.Receiver, error) {
	if err := em.checkInstanceDefaults(event.InstanceID); err != nil {
		return nil, errors.Wrapf(err, "failed to check receiver defaults for instance %s", event.InstanceID)
	}
	receivers := []types.Receiver{}
	// In the below query, note the array_remove to transform [null] to [] if there are no matching rows.
	rows, err := em.DB.Query(
		"get_receivers_for_event",
		`SELECT r.receiver_id, r.receiver_type, r.instance_id, r.address_data,
			array_remove(array_agg(rt.event_type), NULL)
		FROM receivers r LEFT JOIN receiver_event_types rt ON (r.receiver_id = rt.receiver_id)
		WHERE instance_id = $1 AND event_type = $2
		GROUP BY r.receiver_id`,
		event.InstanceID,
		event.Type,
	)
	if err != nil {
		return nil, errors.Wrap(err, "cannot select receivers for event")
	}
	err = em.forEachRow(rows, func(row *sql.Rows) error {
		r, err := types.ReceiverFromRow(row)
		if err != nil {
			return err
		}
		receivers = append(receivers, r)
		return nil
	})
	return receivers, err
}

// CreateEvent inserts event in DB
func (em *EventManager) CreateEvent(event types.Event) error {
	var eventID string
	// Re-encode the message data because the sql driver doesn't understand json columns
	encodedMessages, err := json.Marshal(event.Messages)
	if err != nil {
		return err // This is a server error, because this round-trip *should* work.
	}

	err = em.withTx("add_event_tx", func(tx *utils.Tx) error {
		row := tx.QueryRow(
			"check_event_type_exists",
			`SELECT 1 FROM event_types WHERE name = $1`,
			event.Type,
		)
		var junk int
		err = row.Scan(&junk)
		if err == sql.ErrNoRows {
			return errors.Errorf("event type %s does not exist", event.Type)
		} else if err != nil {
			return err
		}

		metadata, err := json.Marshal(event.Metadata)

		if err != nil {
			return err
		}

		err = tx.QueryRow(
			"add_event",
			`INSERT INTO events (
				event_type,
				instance_id,
				timestamp,
				messages,
				text,
				metadata
			) VALUES ($1, $2, $3, $4, $5, $6) RETURNING event_id
			`,
			event.Type,
			event.InstanceID,
			event.Timestamp,
			encodedMessages,
			event.Text,
			metadata,
		).Scan(&eventID)

		// Save attachments
		for _, attachment := range event.Attachments {
			_, err = tx.Exec("add_attachment",
				`INSERT INTO attachments (
					event_id,
					format,
					body
				) VALUES ($1, $2, $3)
			`,
				eventID,
				attachment.Format,
				attachment.Body,
			)
		}

		return err
	})
	if err != nil {
		return errors.Wrap(err, "cannot insert event")
	}
	return nil
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
	var eventType []string

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
		eventType = strings.Split(params.Get("event_type"), ",")
	}
	if params.Get("fields") != "" {
		var fieldsMap map[string]struct{}
		for _, f := range fields {
			fieldsMap[f] = struct{}{}
		}

		newFields := strings.Split(params.Get("fields"), ",")
		for _, nf := range newFields {
			if _, ok := fieldsMap[nf]; !ok {
				return nil, http.StatusBadRequest, fmt.Errorf("%s is an invalid field", nf)
			}
		}

		fields = newFields
	}

	// Create query
	queryFields := make([]string, len(fields))
	copy(queryFields, fields)
	for i, f := range queryFields {
		queryFields[i] = fmt.Sprintf("e.%s", f) // prepend "e." for join
	}
	queryFields = append(queryFields, "COALESCE(json_agg(a) FILTER (WHERE a.event_id IS NOT NULL), '[]')")
	psql := sq.StatementBuilder.PlaceholderFormat(sq.Dollar)
	query := psql.Select(queryFields...).
		From("events e").
		LeftJoin("attachments a ON (a.event_id = e.event_id)").
		Where(sq.Eq{"instance_id": instanceID}).
		Where(sq.Lt{"timestamp": before}).
		Where(sq.Gt{"timestamp": after}).
		GroupBy("e.event_id").
		OrderBy("timestamp DESC").
		Limit(uint64(limit)).
		Offset(uint64(offset))

	if len(eventType) > 0 {
		query = query.Where(sq.Eq{"event_type": eventType})
	}

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, 0, err
	}
	rows, err := em.DB.Query("get_events", queryString, args...)
	if err != nil {
		return nil, 0, err
	}
	events := []*types.Event{}
	err = em.forEachRow(rows, func(row *sql.Rows) error {
		e, err := types.EventFromRow(row, fields)
		if err != nil {
			return err
		}
		events = append(events, e)
		return nil
	})
	return events, http.StatusOK, err
}

// SyncEventTypes synchronize event types
func (em *EventManager) SyncEventTypes(eventTypes map[string]types.EventType) error {
	return em.withTx("sync_event_types_tx", func(tx *utils.Tx) error {
		oldEventTypes, err := em.listEventTypes(tx, nil)
		if err != nil {
			return err
		}
		for _, oldEventType := range oldEventTypes {
			if eventType, ok := eventTypes[oldEventType.Name]; ok {
				// We delete the entries as we see them so we know at the end what ones are completely new
				delete(eventTypes, eventType.Name)
				if !eventType.Equals(oldEventType) {
					log.Infof("Updating event type %s", eventType.Name)
					err = em.updateEventType(tx, eventType)
					if err != nil {
						return err
					}
				}
			} else {
				log.Warnf("Refusing to delete old event type %s for safety, if you really meant to do this then do so manually", oldEventType.Name)
			}
		}
		// Now create any new types
		for _, eventType := range eventTypes {
			log.Infof("Creating new event type %s", eventType.Name)
			err = em.createEventType(tx, eventType)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func (em *EventManager) createEventType(tx *utils.Tx, e types.EventType) error {
	// Since go interprets omitted as empty string for feature flag, translate empty string to NULL on insert.
	result, err := tx.Exec(
		"create_event_type",
		`INSERT INTO event_types (name, display_name, description, default_receiver_types, feature_flag)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''))
		ON CONFLICT DO NOTHING`,
		e.Name,
		e.DisplayName,
		e.Description,
		pq.Array(e.DefaultReceiverTypes),
		e.FeatureFlag,
	)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return fmt.Errorf("Event already exists")
	}
	// Now add default configs for receivers
	_, err = tx.Exec(
		"set_receiver_defaults_for_new_event_type",
		`INSERT INTO receiver_event_types (receiver_id, event_type) (
			SELECT receiver_id, $1 FROM receivers
			WHERE receiver_type = ANY ($2)
		)`,
		e.Name,
		pq.Array(e.DefaultReceiverTypes),
	)
	return err
}

func (em *EventManager) updateEventType(tx *utils.Tx, e types.EventType) error {
	// Since go interprets omitted as empty string for feature flag, translate empty string to NULL on insert.
	result, err := tx.Exec(
		"update_event_type",
		`UPDATE event_types
		SET (display_name, description, default_receiver_types, feature_flag) = ($2, $3, $4, NULLIF($5, ''))
		WHERE name = $1`,
		e.Name,
		e.DisplayName,
		e.Description,
		pq.Array(e.DefaultReceiverTypes),
		e.FeatureFlag,
	)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return fmt.Errorf("Event type does not exist")
	}
	return nil
}

func (em *EventManager) deleteEventType(tx *utils.Tx, eventTypeName string) error {
	result, err := em.DB.Exec(
		"delete_event_type",
		`DELETE FROM event_types
		WHERE name = $1`,
		eventTypeName,
	)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return fmt.Errorf("Event type does not exist")
	}
	return nil
}
