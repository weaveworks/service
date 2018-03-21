package sql

import (
	"database/sql"
	"encoding/json"

	_ "github.com/lib/pq" // initialises the postgres driver
	"github.com/pkg/errors"

	"github.com/weaveworks/service/flux-api/instance"
	"github.com/weaveworks/service/flux-api/service"
)

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

// UpdateConnection updates the connection for the given instanceID.
func (db *DB) UpdateConnection(inst service.InstanceID, update instance.UpdateFunc) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}

	// TODO: figure out how to unmarshal into `instance.Connection` safely without
	// breaking any Weave Cloud behaviour. Specifically we need to preserve the
	// `Connected` field because some UI behaviour might depend on it.
	var (
		currentConfig instance.Config
		confString    string
	)
	switch tx.QueryRow(`SELECT config FROM config WHERE instance = $1`, string(inst)).Scan(&confString) {
	case sql.ErrNoRows:
		currentConfig = instance.Config{}
	case nil:
		if err = json.Unmarshal([]byte(confString), &currentConfig); err != nil {
			return err
		}
	default:
		return err
	}

	var newConfig instance.Config
	newConfig.Connection, err = update(currentConfig.Connection)
	if err != nil {
		err2 := tx.Rollback()
		if err2 != nil {
			return errors.Wrapf(err, "transaction rollback failed: %s", err2)
		}
		return err
	}

	newConfigBytes, err := json.Marshal(newConfig)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`DELETE FROM config WHERE instance = $1`, string(inst))
	if err == nil {
		_, err = tx.Exec(`INSERT INTO config (instance, config, stamp) VALUES
                       ($1, $2, now())`, string(inst), string(newConfigBytes))
	}
	if err == nil {
		err = tx.Commit()
	}
	return err
}

// GetConnection gets the connection for the given instanceID.
func (db *DB) GetConnection(inst service.InstanceID) (instance.Connection, error) {
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
	return conf.Connection, json.Unmarshal([]byte(c), &conf)
}

// ---

func (db *DB) sanityCheck() error {
	_, err := db.conn.Query(`SELECT instance, config, stamp FROM config LIMIT 1`)
	if err != nil {
		return errors.Wrap(err, "failed sanity check for config table")
	}
	return nil
}
