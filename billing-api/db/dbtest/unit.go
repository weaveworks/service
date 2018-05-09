// +build !integration

package dbtest

import (
	"context"
	"flag"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/weaveworks/service/billing-api/db"
	"github.com/weaveworks/service/common/dbconfig"
)

var (
	databaseURI        = flag.String("database-uri", "memory://", "Uri of a test database")
	databaseMigrations = flag.String("database-migrations", "", "Path where the database migration files can be found")
)

// Setup sets up stuff for testing, creating a new database
func Setup(t *testing.T) db.DB {
	database, err := db.New(dbconfig.New(*databaseURI, *databaseMigrations, ""))
	require.NoError(t, err)
	return database
}

// Cleanup cleans up after a test
func Cleanup(t *testing.T, database db.DB) {
	require.NoError(t, database.Close(context.Background()))
}
