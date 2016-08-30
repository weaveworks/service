// +build integration

package test

import (
	"flag"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/weaveworks/service/users/storage"
)

var (
	databaseURI        = flag.String("database-uri", "postgres://postgres@users-db.weave.local/users_test?sslmode=disable", "Uri of a test database")
	databaseMigrations = flag.String("database-migrations", "/migrations", "Path where the database migration files can be found")
)

// Setup sets up stuff for testing, creating a new database
func Setup(t *testing.T) storage.Database {
	storage.PasswordHashingCost = bcrypt.MinCost
	db := storage.MustNew(*databaseURI, *databaseMigrations)
	require.NoError(t, db.(storage.Truncater).Truncate())
	return db
}

// Cleanup cleans up after a test
func Cleanup(t *testing.T, db storage.Database) {
	require.NoError(t, db.Close())
}
