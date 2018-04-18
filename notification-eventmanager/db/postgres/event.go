package postgres

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/weaveworks/service/notification-eventmanager/types"
	"github.com/weaveworks/service/notification-eventmanager/utils"

	_ "github.com/lib/pq"                          // Import the postgres sql driver
	_ "gopkg.in/mattes/migrate.v1/driver/postgres" // Import the postgres migrations driver
)

// CreateEvent inserts event in DB
func (d DB) CreateEvent(event types.Event, featureFlags []string) error {
	var eventID string
	// Re-encode the message data because the sql driver doesn't understand json columns
	encodedMessages, err := json.Marshal(event.Messages)
	if err != nil {
		return err // This is a server error, because this round-trip *should* work.
	}

	err = d.withTx("add_event_tx", func(tx *utils.Tx) error {
		row := tx.QueryRow(
			"check_event_type_exists",
			`SELECT feature_flag FROM event_types WHERE name = $1`,
			event.Type,
		)
		var wantFlag sql.NullString
		err = row.Scan(&wantFlag)
		if err == sql.ErrNoRows {
			return errors.Errorf("event type %s does not exist", event.Type)
		} else if err != nil {
			return err
		}
		// If instance does not have necessary feature flag, just skip this event.
		// This is not an error because the sender might not necessarily know that
		// this instance is missing the feature flag.
		if wantFlag.String != "" {
			hasFlag := false
			for _, flag := range featureFlags {
				if flag == wantFlag.String {
					hasFlag = true
				}
			}
			if !hasFlag {
				log.Infof("Skipping event `%s` for missing feature flag `%s`", event.Type, wantFlag.String)
				return nil
			}
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

// GetEvents returns list of events
func (d DB) GetEvents(instanceID string, fields, eventTypes []string, before, after time.Time, limit, offset int) ([]*types.Event, error) {
	// Create query
	queryFields := make([]string, len(fields))
	copy(queryFields, fields)
	for i, f := range queryFields {
		queryFields[i] = fmt.Sprintf("e.%s", f) // prepend "e." for join
	}
	queryFields = append(queryFields, "COALESCE(json_agg(a) FILTER (WHERE a.event_id IS NOT NULL), '[]')")
	psql := sq.StatementBuilder.PlaceholderFormat(sq.Dollar)
	// Use squirrel to abstract away from writing specific postgreSQL (conditional WHERE clause)
	// We are operating on an object. With a raw SQL query string, interpolating a WHERE clause depending on a condition becomes messy quickly
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

	if len(eventTypes) > 0 {
		query = query.Where(sq.Eq{"event_type": eventTypes})
	}

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}
	rows, err := d.client.Query("get_events", queryString, args...)
	if err != nil {
		return nil, err
	}
	events := []*types.Event{}
	err = forEachRow(rows, func(row *sql.Rows) error {
		e, err := types.EventFromRow(row, fields)
		if err != nil {
			return err
		}
		events = append(events, e)
		return nil
	})
	return events, err
}
