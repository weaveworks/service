package sql

import (
	"database/sql"
	"encoding/json"
	"time"

	_ "github.com/lib/pq" // initialises the postgres driver
	"github.com/pkg/errors"

	"github.com/weaveworks/service/flux-api/instance"
	"github.com/weaveworks/service/flux-api/service"
)

// TODO: test this package against postgres after as part of #1894.
// Not really worth doing it before that point.

// DB is an Instance DB
type DB struct {
	conn *sql.DB
}

// New creates a new DB.
func New(driver, datasource string) (*DB, error) {
	conn, err := sql.Open(driver, datasource)
	if err != nil {
		return nil, err
	}
	db := &DB{
		conn: conn,
	}
	return db, db.sanityCheck()
}

// Get gets connection information for the given instance.
func (db *DB) Get(inst service.InstanceID) (instance.Connection, error) {
	var c string
	err := db.conn.QueryRow(`SELECT config FROM config WHERE instance = $1`, string(inst)).Scan(&c)
	switch err {
	case nil:
		break
	case sql.ErrNoRows:
		return instance.Connection{}, nil
	default:
		return instance.Connection{}, err
	}
	var conf instance.Config
	err = json.Unmarshal([]byte(c), &conf)
	return conf.Connection, err
}

// Connect records connection time for the given instance.
func (db *DB) Connect(inst service.InstanceID, t time.Time) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}

	// TODO: store an `instance.Connection` instead
	var config instance.Config
	config.Connection.Last = t
	config.Connection.Connected = true

	configBytes, err := json.Marshal(config)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT INTO config (instance, config, stamp)
					  VALUES ($1, $2, now())
					  ON CONFLICT DO UPDATE`, string(inst), string(configBytes))
	if err != nil {
		return tx.Rollback()
	}
	return tx.Commit()
}

// Disconnect records disconnection for the given instance, if and only if
// the existing connection timestamp is equal to `t`. This prevents a
// daemon being erroneously marked as disconnected in the case that it is
// able to quickly reconnect before this method executes.
func (db *DB) Disconnect(inst service.InstanceID, t time.Time) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}

	// TODO: store an `instance.Connection` instead
	var expectedConfig instance.Config
	expectedConfig.Connection.Last = t
	expectedConfig.Connection.Connected = true

	newConfig := expectedConfig
	newConfig.Connection.Connected = false

	expectedConfigBytes, err := json.Marshal(expectedConfig)
	if err != nil {
		return err
	}
	newConfigBytes, err := json.Marshal(newConfig)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`UPDATE config
					  SET (config, stamp) = ($3, now())
					  WHERE instance = $1 AND config = $2`,
		string(inst), string(expectedConfigBytes), string(newConfigBytes))
	if err != nil {
		return tx.Rollback()
	}
	return tx.Commit()
}

func (db *DB) sanityCheck() error {
	_, err := db.conn.Query(`SELECT instance, config, stamp FROM config LIMIT 1`)
	if err != nil {
		return errors.Wrap(err, "failed sanity check for config table")
	}
	return nil
}
