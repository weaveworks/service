package postgres

import (
	"database/sql"

	"github.com/pkg/errors"

	"github.com/lib/pq"
	log "github.com/sirupsen/logrus"
	"github.com/weaveworks/service/notification-eventmanager/types"
	"github.com/weaveworks/service/notification-eventmanager/utils"

	_ "github.com/lib/pq"                          // Import the postgres sql driver
	_ "gopkg.in/mattes/migrate.v1/driver/postgres" // Import the postgres migrations driver
)

// ListEventTypes returns a list of event types from the DB, either using given transaction or without a transaction if nil.
// Also filter by enabled feature flags if featureFlags is not nil.
func (d DB) ListEventTypes(tx *utils.Tx, featureFlags []string) ([]types.EventType, error) {
	queryFn := d.client.Query
	if tx != nil {
		queryFn = tx.Query
	}
	// Exclude feature-flagged rows only if a) feature flags given is not nil and b) row has a feature flag
	// and c) feature flag isn't in given list of feature flags.
	rows, err := queryFn(
		"list_event_types",
		`SELECT name, display_name, description, default_receiver_types, hide_ui_config, feature_flag, hidden_receiver_types FROM event_types
		WHERE $1::text[] IS NULL OR feature_flag IS NULL OR feature_flag = ANY ($1::text[])`,
		pq.Array(featureFlags),
	)
	if err != nil {
		return nil, err
	}
	eventTypes := []types.EventType{}
	err = forEachRow(rows, func(row *sql.Rows) error {
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

// SyncEventTypes updates the event types in the database.
func (d DB) SyncEventTypes(eventTypes map[string]types.EventType) error {
	return d.withTx("sync_event_types_tx", func(tx *utils.Tx) error {
		oldEventTypes, err := d.ListEventTypes(tx, nil)
		if err != nil {
			return err
		}
		for _, oldEventType := range oldEventTypes {
			if eventType, ok := eventTypes[oldEventType.Name]; ok {
				// We delete the entries as we see them so we know at the end what ones are completely new
				delete(eventTypes, eventType.Name)
				if !eventType.Equals(oldEventType) {
					log.Infof("Updating event type %s", eventType.Name)
					err = d.UpdateEventType(tx, eventType)
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
			err = d.CreateEventType(tx, eventType)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// CreateEventType creates new event type
func (d DB) CreateEventType(tx *utils.Tx, e types.EventType) error {
	// Since go interprets omitted as empty string for feature flag, translate empty string to NULL on insert.
	result, err := tx.Exec(
		"create_event_type",
		`INSERT INTO event_types (name, display_name, description, default_receiver_types, hide_ui_config, feature_flag)
		VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''))
		ON CONFLICT DO NOTHING`,
		e.Name,
		e.DisplayName,
		e.Description,
		pq.Array(e.DefaultReceiverTypes),
		e.HideUIConfig,
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
		return errors.New("Event type already exists")
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

// UpdateEventType updates event type
func (d DB) UpdateEventType(tx *utils.Tx, e types.EventType) error {
	// Since go interprets omitted as empty string for feature flag, translate empty string to NULL on insert.
	result, err := tx.Exec(
		"update_event_type",
		`UPDATE event_types
		SET (display_name, description, default_receiver_types, hide_ui_config, feature_flag, hidden_receiver_types) = ($2, $3, $4, $5, NULLIF($6, ''), $7)
		WHERE name = $1`,
		e.Name,
		e.DisplayName,
		e.Description,
		pq.Array(e.DefaultReceiverTypes),
		e.HideUIConfig,
		e.FeatureFlag,
		pq.Array(e.HiddenReceiverTypes),
	)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return errors.New("Event type does not exist")
	}
	return nil
}

// DeleteEventType deletes event type
func (d DB) DeleteEventType(tx *utils.Tx, eventTypeName string) error {
	result, err := d.client.Exec(
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
		return errors.New("Event type does not exist")
	}
	return nil
}
