// Code for initialising databases; individual components should put
// scripts in `db/migrations/{driver}`.  `db.MustMigrate` can then be
// used to make sure the database is up to date before components can
// use it.

package db

import (
	"net/url"
	"os"
	"strings"

	"github.com/pkg/errors"
	"gopkg.in/mattes/migrate.v1/migrate"

	// This section imports the data/sql drivers and the migration
	// drivers.
	_ "github.com/lib/pq"
	_ "gopkg.in/mattes/migrate.v1/driver/postgres"
)

// Migrate makes sure the database at the URL is up to date with respect
// to  migrations, or return an error. The migration scripts are taken
// from `basedir/{scheme}`, with the scheme coming from the URL.
func Migrate(dbURL, migrationsPath string) (uint64, error) {
	u, err := url.Parse(dbURL)
	if err != nil {
		return 0, errors.Wrap(err, "parsing database URL")
	}
	if _, err := os.Stat(migrationsPath); err != nil {
		if os.IsNotExist(err) {
			return 0, errors.Wrapf(err, "migrations dir %s does not exist; driver %s not supported", migrationsPath, u.Scheme)
		}
		return 0, errors.Wrapf(err, "verifying migrations directory %s exists", migrationsPath)
	}

	errs, _ := migrate.UpSync(dbURL, migrationsPath)
	if len(errs) > 0 {
		return 0, errors.Wrap(compositeError{errs}, "migrating database")
	}
	version, err := migrate.Version(dbURL, migrationsPath)
	if err != nil {
		return 0, err
	}
	return version, nil
}

type compositeError struct {
	errors []error
}

func (errs compositeError) Error() string {
	msgs := make([]string, len(errs.errors))
	for i := range errs.errors {
		msgs[i] = errs.errors[i].Error()
	}
	return strings.Join(msgs, "; ")
}
