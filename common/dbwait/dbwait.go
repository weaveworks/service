package dbwait

import (
	"database/sql"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

// timeout waiting for database connection to be established
const timeout = 5 * time.Minute

// Wait waits for database connection to be established
func Wait(db *sql.DB) error {
	deadline := time.Now().Add(timeout)
	var err error
	for tries := 0; time.Now().Before(deadline); tries++ {
		err = db.Ping()
		if err == nil {
			return nil
		}
		log.Debugf("db connection not established, error: %s; retrying...", err)
		time.Sleep(time.Second << uint(tries))
	}
	return errors.Wrapf(err, "db connection not established after %s", timeout)
}
