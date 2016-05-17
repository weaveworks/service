// +build integration

package main

import (
	"flag"
	"testing"

	"github.com/Masterminds/squirrel"
	"github.com/jordan-wright/email"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/weaveworks/service/users/login"
)

var (
	databaseURI        = flag.String("database-uri", "postgres://postgres@users-db.weave.local/users_test?sslmode=disable", "Uri of a test database")
	databaseMigrations = flag.String("database-migrations", "/migrations", "Path where the database migration files can be found")

	sentEmails []*email.Email
	app        *api
	storage    database
	logins     *login.Providers
	sessions   sessionStore
	domain     = "http://fake.scope"
)

func setup(t *testing.T) {
	passwordHashingCost = bcrypt.MinCost

	// TODO: Use some more realistic mailer here
	sentEmails = nil

	var directLogin = false

	setupLogging("debug")
	storage = mustNewDatabase(*databaseURI, *databaseMigrations)
	sessions = mustNewSessionStore("Test-Session-Secret-Which-Is-64-Bytes-Long-aa1a166556cb719f531cd", storage)
	templates := mustNewTemplateEngine()
	logins = login.NewProviders()

	truncateDatabase(t)

	emailer := smtpEmailer{templates, testEmailSender, domain, "test@test.com"}
	app = newAPI(directLogin, emailer, sessions, storage, logins, templates)
}

func cleanup(t *testing.T) {
	logins.Reset()
	require.NoError(t, storage.Close())
}

func testEmailSender(e *email.Email) error {
	sentEmails = append(sentEmails, e)
	return nil
}

// Truncate the test store. Assumes the db is Postgres.
func truncateDatabase(t *testing.T) {
	db := storage.(squirrel.Execer)
	mustExec(t, db, `truncate table traceable;`)
	mustExec(t, db, `truncate table users;`)
	mustExec(t, db, `truncate table logins;`)

	mustExec(t, db, `truncate table organizations;`)
	mustExec(t, db, `truncate table memberships;`)
}

func mustExec(t *testing.T, db squirrel.Execer, query string, args ...interface{}) {
	_, err := db.Exec(query, args...)
	require.NoError(t, err)
}
