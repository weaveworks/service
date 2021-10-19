//go:build !integration
// +build !integration

package dbtest

import (
	"context"
	"flag"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/weaveworks/service/common/dbconfig"
	"github.com/weaveworks/service/users/db"
)

var (
	databaseURI        = flag.String("database-uri", "memory://", "Uri of a test database")
	databaseMigrations = flag.String("database-migrations", "", "Path where the database migration files can be found")
)

// Setup sets up stuff for testing, creating a new database
func Setup(t *testing.T) db.DB {
	db.PasswordHashingCost = bcrypt.MinCost
	database := db.MustNew(dbconfig.New(*databaseURI, *databaseMigrations, ""))
	return database
}

// Cleanup cleans up after a test
func Cleanup(t *testing.T, database db.DB) {
	require.NoError(t, database.Close(context.Background()))
}
