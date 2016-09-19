// +build integration

package dbtest

import (
	"flag"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/weaveworks/service/users/db"
	_ "github.com/weaveworks/service/users/db/memory"   // Load the memory db driver
	_ "github.com/weaveworks/service/users/db/postgres" // Load the postgres db driver
)

var (
	databaseURI        = flag.String("database-uri", "postgres://postgres@users-db.weave.local/users_test?sslmode=disable", "Uri of a test database")
	databaseMigrations = flag.String("database-migrations", "/migrations", "Path where the database migration files can be found")
)

// Setup sets up stuff for testing, creating a new database
func Setup(t *testing.T) db.DB {
	db.PasswordHashingCost = bcrypt.MinCost
	database := db.MustNew(*databaseURI, *databaseMigrations)
	require.NoError(t, database.(db.Truncater).Truncate())
	return database
}

// Cleanup cleans up after a test
func Cleanup(t *testing.T, database db.DB) {
	require.NoError(t, database.Close())
}
