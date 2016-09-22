// +build integration

package dbtest

import "flag"

var (
	databaseURI        = flag.String("database-uri", "postgres://postgres@users-db.weave.local/users_test?sslmode=disable", "Uri of a test database")
	databaseMigrations = flag.String("database-migrations", "/migrations", "Path where the database migration files can be found")
)
