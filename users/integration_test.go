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
)

var (
	databaseURI = flag.String("database-uri", "postgres://postgres@users-db.weave.local/users_test?sslmode=disable", "Uri of a test database")

	sentEmails []*email.Email
	app        *api
	storage    database
	sessions   sessionStore
)

func setup(t *testing.T) {
	passwordHashingCost = bcrypt.MinCost

	// TODO: Use some more realistic mailer here
	sentEmails = nil

	var directLogin = false

	setupLogging("debug")
	storage = mustNewDatabase(*databaseURI)
	sessions = mustNewSessionStore("Test-Session-Secret-Which-Is-64-Bytes-Long-aa1a166556cb719f531cd", storage)
	templates := mustNewTemplateEngine()

	truncateDatabase(t)

	app = newAPI(directLogin, testEmailSender, sessions, storage, templates)
}

func cleanup(t *testing.T) {
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
	mustExec(t, db, `truncate table organizations;`)
}

func mustExec(t *testing.T, db squirrel.Execer, query string, args ...interface{}) {
	_, err := db.Exec(query, args...)
	require.NoError(t, err)
}
