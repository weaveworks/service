// +build !integration

package dbtest

import "flag"

var (
	databaseURI        = flag.String("database-uri", "memory://", "Uri of a test database")
	databaseMigrations = flag.String("database-migrations", "", "Path where the database migration files can be found")
)
