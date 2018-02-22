package postgres

import (
	"database/sql"
	"encoding/json"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/weaveworks/service/notification-eventmanager/types"
	"github.com/weaveworks/service/notification-eventmanager/utils"
	"strings"
)

var databaseRequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
	Namespace: "notification",
	Name:      "database_request_duration_seconds",
	Help:      "Time spent (in seconds) doing database requests.",
	Buckets:   prometheus.DefBuckets,
}, []string{"method", "status_code"})

type DB struct {
	client *utils.DB
}

func New(uri, migrationsDir string) (DB, error) {
	db, err := sql.Open("postgres", uri)

	if err != nil {
		return DB{}, err
	}

	d := utils.NewDB(db, databaseRequestDuration)

	return DB{client: d}, nil
}

// Executes the given function in a transaction, and rolls back or commits depending on if the function errors.
// Ignores errors from rollback.
func (d DB) withTx(method string, f func(tx *utils.Tx) error) error {
	tx, err := d.client.Begin(method)
	if err != nil {
		return err
	}
	err = f(tx)
	if err == nil {
		tx.Commit()
		return err
	}
	_ = tx.Rollback()

	if strings.Contains(err.Error(), "pq: duplicate key value violates unique constraint \"instances_initialized_pkey\"") {
		// if error is pq: duplicate key value violates unique constraint "instances_initialized_pkey"
		// instance already initialized, ignore this error
		return nil
	}

	return err
}

// for each row returned from query, calls callback
func forEachRow(rows *sql.Rows, callback func(*sql.Rows) error) error {
	var err error
	for err == nil && rows.Next() {
		err = callback(rows)
	}
	if err == nil {
		err = rows.Err()
	}
	return err
}

func (d DB) SaveAttachments(event_id string, a []types.Attachment) error {
	err := d.withTx("add_attachments_tx", func(tx *utils.Tx) error {
		var err error
		for _, attachment := range a {
			row := tx.QueryRow("add_attachment",
				`INSERT INTO attachments (
					event_id,
					format,
					body,
				) VALUES $1, $2, $3`,
				event_id,
				attachment.Format,
				attachment.Body,
			)
			var junk int
			err = row.Scan(&junk)
		}
		return err
	})

	return err
}

func (d DB) CreateEvent(event types.Event) (string, error) {
	id := ""
	err := d.withTx("add_event_tx", func(tx *utils.Tx) error {
		row := tx.QueryRow(
			"check_event_type_exists",
			`SELECT 1 FROM event_types WHERE name = $1`,
			event.Type,
		)
		var junk int
		err := row.Scan(&junk)
		if err == sql.ErrNoRows {
			return errors.Errorf("event type %s does not exist", event.Type)
		} else if err != nil {
			return err
		}

		metadata, err := json.Marshal(event.Metadata)

		if err != nil {
			return err
		}

		tx.QueryRow(
			"add_event",
			`INSERT INTO events (
				event_type,
				instance_id,
				timestamp,
				fallback,
				html,
				metadata
			) VALUES ($1, $2, $3, $4, $5, $6) RETURNING event_id`,
			event.Type,
			event.InstanceID,
			event.Timestamp,
			event.Fallback,
			event.HTML,
			metadata,
		).Scan(&id)

		err = d.SaveAttachments(id, event.Attachments)
		return err
	})

	return id, err
}

func (d DB) ListEvents(instanceID string, offset, length int) ([]types.Event, error) {
	rows, err := d.client.Query(
		"get_events",
		`SELECT
			e.event_id,
			e.event_type,
			e.instance_id,
			e.timestamp,
			e.fallback,
			e.html,
			e.metadata,
			e.messages,
			COALESCE(json_agg(a) FILTER (WHERE a.event_id IS NOT NULL), '[]')

		FROM events e
		LEFT JOIN attachments a ON (a.event_id = e.event_id)
		WHERE e.instance_id = $1
		GROUP BY e.event_id
		ORDER BY timestamp DESC
		LIMIT $2 OFFSET $3
		`,
		instanceID,
		length,
		offset,
	)
	if err != nil {
		return nil, err
	}
	events := []types.Event{}
	err = forEachRow(rows, func(row *sql.Rows) error {
		e, err := types.EventFromRow(row)
		if err != nil {
			return err
		}
		events = append(events, e)
		return nil
	})

	return events, err
}
