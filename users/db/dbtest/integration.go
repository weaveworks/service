// +build integration

package dbtest

import (
	"context"
	"flag"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/weaveworks/common/logging"
	"github.com/weaveworks/service/users/db"
	"github.com/weaveworks/service/users/db/postgres"
)

var (
	databaseURI        = flag.String("database-uri", "postgres://postgres@users-db.weave.local/users_test?sslmode=disable", "Uri of a test database")
	databaseMigrations = flag.String("database-migrations", "/migrations", "Path where the database migration files can be found")

	done        chan error
	errRollback = fmt.Errorf("Rolling back test data")
)

// Setup sets up stuff for testing, creating a new database
func Setup(t *testing.T) db.DB {
	require.NoError(t, logging.Setup("debug"))
	db.PasswordHashingCost = bcrypt.MinCost
	// Don't use db.MustNew, here so we can do a transaction around the whole test, to rollback.
	pg, err := postgres.New(*databaseURI, *databaseMigrations, bcrypt.MinCost)
	require.NoError(t, err)

	newDB := make(chan db.DB)
	done = make(chan error)
	go func() {
		done <- pg.Transaction(func(tx postgres.DB) error {
			// Pass out the tx so we can run the test
			newDB <- tx
			// Wait for the test to finish
			return <-done
		})
	}()
	// Get the new database
	return <-newDB
}

// Cleanup cleans up after a test
func Cleanup(t *testing.T, database db.DB) {
	if done != nil {
		done <- errRollback
		require.Equal(t, errRollback, <-done)
		done = nil
	}
	require.NoError(t, database.Close(context.Background()))
}
