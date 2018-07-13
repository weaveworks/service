// +build integration

package dbtest

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/weaveworks/service/billing-api/db"
	"github.com/weaveworks/service/common/dbconfig"
	"github.com/weaveworks/service/common/file"
)

var (
	databaseURI        = flag.String("database-uri", "postgres://postgres@billing-db.weave.local/billing_test?sslmode=disable", "Uri of a test database")
	databaseMigrations = flag.String("database-migrations", "/migrations", "Path where the database migration files can be found")

	lockFile = filepath.Join(os.TempDir(), "billing-test-db-migrations.lck")

	done        chan error
	errRollback = fmt.Errorf("Rolling back test data")
)

const maxAttempts = 5

// Setup sets up stuff for testing, creating a new database
func Setup(t *testing.T) db.DB {
	lockfile, err := file.Lock(lockFile, maxAttempts)
	require.NoError(t, err)
	defer lockfile.Unlock()

	pg, err := db.New(dbconfig.New(*databaseURI, *databaseMigrations, ""))
	require.NoError(t, err)

	newDB := make(chan db.DB)
	done = make(chan error)
	go func() {
		done <- pg.Transaction(func(tx db.DB) error {
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
