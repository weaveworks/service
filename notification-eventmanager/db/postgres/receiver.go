package postgres

import (
	"database/sql"
	"encoding/json"
	"strings"

	"github.com/pkg/errors"

	"github.com/lib/pq"
	"github.com/weaveworks/service/notification-eventmanager/types"
	"github.com/weaveworks/service/notification-eventmanager/utils"

	_ "github.com/lib/pq"                          // Import the postgres sql driver
	_ "gopkg.in/mattes/migrate.v1/driver/postgres" // Import the postgres migrations driver
)

// Called before any handlers involving receivers, to initialize receiver defaults for the instance
// if it hasn't been already.
func (d DB) checkInstanceDefaults(instanceID string) error {
	return d.withTx("check_instance_defaults_tx", func(tx *utils.Tx) error {
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
		if _, err := d.createReceiverTX(receiver, instanceID); err != nil {
			return err
		}

		// finally, insert instance_id into instances_initialized to mark it as done
		_, err := tx.Exec("set_instance_initialized", "INSERT INTO instances_initialized (instance_id) VALUES ($1)", instanceID)
		return err
	})
}

// ListReceivers returns a list of receivers
func (d DB) ListReceivers(instanceID string) ([]types.Receiver, error) {
	if err := d.checkInstanceDefaults(instanceID); err != nil {
		return nil, err
	}
	// In the below query, note the array_remove to transform [null] to [] if there are no matching rows.
	// Note we exclude event types with non-matching feature flags.
	rows, err := d.client.Query(
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
		return nil, err
	}
	receivers := []types.Receiver{}
	err = forEachRow(rows, func(row *sql.Rows) error {
		r, err := types.ReceiverFromRow(row)
		if err != nil {
			return err
		}
		receivers = append(receivers, r)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return receivers, nil
}

// CreateReceiver creates receiver with check for defaults receivers for instance
func (d DB) CreateReceiver(receiver types.Receiver, instanceID string) (string, error) {
	if err := d.checkInstanceDefaults(instanceID); err != nil {
		return "", err
	}
	receiverID, err := d.createReceiverTX(receiver, instanceID)
	if err != nil {
		return "", err
	}
	return receiverID, nil
}

// createReceiverTX creates receiver without check for defaults receivers
func (d DB) createReceiverTX(receiver types.Receiver, instanceID string) (string, error) {
	// Re-encode the address data because the sql driver doesn't understand json columns
	// TODO validate this field against the specific receiver type
	encodedAddress, err := json.Marshal(receiver.AddressData)
	if err != nil {
		return "", err // This is a server error, because this round-trip *should* work.
	}

	var receiverID string
	var userError error

	err = d.withTx("create_receiver_tx", func(tx *utils.Tx) error {
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
		err = row.Scan(&receiverID)
		if err != nil {
			userError = err
			return err
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
			userError = err
			return err
		}
		return nil
	})
	return receiverID, userError
}

// GetReceiver returns a receiver
func (d DB) GetReceiver(instanceID string, receiverID string, featureFlags []string, omitHiddenEventTypes bool) (types.Receiver, error) {
	if err := d.checkInstanceDefaults(instanceID); err != nil {
		return types.Receiver{}, err
	}

	// In the below query, note the array_remove to transform [null] to [] if there are no matching rows.
	row := d.client.QueryRow(
		"get_receiver",
		`SELECT r.receiver_id, r.receiver_type, r.instance_id, r.address_data,
			array_remove(array_agg(rt.event_type), NULL)
		FROM receivers r
		LEFT JOIN receiver_event_types rt ON (r.receiver_id = rt.receiver_id)
		LEFT JOIN event_types et ON (rt.event_type = et.name)
		WHERE r.receiver_id = $1 AND r.instance_id = $2 AND (et.feature_flag IS NULL OR et.feature_flag = ANY ($3))
			AND ($4 = false OR et.hide_ui_config = false)
		GROUP BY r.receiver_id`,
		receiverID,
		instanceID,
		pq.Array(featureFlags),
		omitHiddenEventTypes,
	)
	receiver, err := types.ReceiverFromRow(row)
	if err != nil {
		return types.Receiver{}, err
	}
	return receiver, nil
}

// UpdateReceiver updates receiver
func (d DB) UpdateReceiver(receiver types.Receiver, instanceID string, featureFlags []string) error {
	if err := d.checkInstanceDefaults(instanceID); err != nil {
		return err
	}
	// Re-encode the address data because the sql driver doesn't understand json columns
	encodedAddress, err := json.Marshal(receiver.AddressData)
	if err != nil {
		return err // This is a server error, because this round-trip *should* work.
	}

	// We need to read some DB values to validate the new data (we could just rely on the DB's constraints, but it's hard
	// to return a meaningful error message if we do that). We do this in a transaction to prevent races.
	// We set these vars as a way of returning more values from inside the closure, since the function can only return error.
	var userError error
	err = d.withTx("update_receiver_tx", func(tx *utils.Tx) error {
		// Verify receiver exists, has correct instance id and type.
		var rtype string
		err := tx.QueryRow("check_receiver_exists", `SELECT receiver_type FROM receivers WHERE receiver_id = $1 AND instance_id = $2`, receiver.ID, instanceID).
			Scan(&rtype)

		if err != nil {
			return err
		}

		// Verify event types list is valid by querying for items in the input but not in event_types
		rows, err := tx.Query(
			"check_new_receiver_event_types",
			`SELECT unnest FROM unnest($1::text[])
			WHERE unnest NOT IN (SELECT name FROM event_types WHERE hide_ui_config <> true)`,
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
			userError = errors.Errorf("Given event types do not exist or cannot be modified: %s", strings.Join(badTypes, ", "))
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
		// Delete any newly-dropped event types. Note we keep feature-flag-hidden event types and those that are hidden from config UI
		// since the client wouldn't have known about these so omitting them was not an intentional delete.
		_, err = tx.Exec(
			"remove_receiver_event_types",
			`DELETE FROM receiver_event_types
			WHERE receiver_id = $1 AND NOT event_type IN (
				SELECT name
				FROM event_types
				WHERE name = ANY ($2) OR NOT (feature_flag IS NULL OR feature_flag = ANY ($3)) OR hide_ui_config = true
			)`,
			receiver.ID,
			pq.Array(receiver.EventTypes),
			pq.Array(featureFlags),
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

	return userError
}

// DeleteReceiver deletes receiver
func (d DB) DeleteReceiver(instanceID string, receiverID string) (int64, error) {
	if err := d.checkInstanceDefaults(instanceID); err != nil {
		return 0, err
	}

	result, err := d.client.Exec(
		"delete_receiver",
		`DELETE FROM receivers
		WHERE receiver_id = $1 AND instance_id = $2`,
		receiverID,
		instanceID,
	)
	if err != nil {
		return 0, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	if affected == 0 {
		return 0, nil
	}
	return affected, nil
}

// GetReceiversForEvent returns all receivers for event from DB
func (d DB) GetReceiversForEvent(event types.Event) ([]types.Receiver, error) {
	if err := d.checkInstanceDefaults(event.InstanceID); err != nil {
		return nil, errors.Wrapf(err, "failed to check receiver defaults for instance %s", event.InstanceID)
	}
	receivers := []types.Receiver{}
	// In the below query, note the array_remove to transform [null] to [] if there are no matching rows.
	rows, err := d.client.Query(
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
	err = forEachRow(rows, func(row *sql.Rows) error {
		r, err := types.ReceiverFromRow(row)
		if err != nil {
			return err
		}
		receivers = append(receivers, r)
		return nil
	})
	return receivers, err
}
