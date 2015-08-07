// +build integration

package main

import (
	"database/sql"
	"flag"
	"net/http"
	"testing"

	"github.com/jordan-wright/email"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

var databaseURI = flag.String("database-uri", "postgres://postgres@users-db.weave.local/weave_test?sslmode=disable", "Uri of a test database")

var sentEmails []*email.Email
var app http.Handler

func setup(t *testing.T) {
	passwordHashingCost = bcrypt.MinCost

	// TODO: Use some more realistic mailer here
	sentEmails = nil
	sendEmail = testEmailSender

	var directLogin = false

	setupLogging("debug")
	setupStorage(*databaseURI)
	setupTemplates()
	setupSessions("Test-Session-Secret-Which-Is-64-Bytes-Long-aa1a166556cb719f531cd")

	truncateDatabase(t)

	app = handler(directLogin)
}

func cleanup(t *testing.T) {
	require.NoError(t, storage.Close())
}

func testEmailSender(e *email.Email) error {
	sentEmails = append(sentEmails, e)
	return nil
}

type Execer interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
}

// Truncate the test store. Assumes the db is Postgres.
func truncateDatabase(t *testing.T) {
	db := storage.(Execer)
	mustExec(t, db, `truncate table traceable;`)
	mustExec(t, db, `truncate table users;`)
	mustExec(t, db, `truncate table organizations;`)
}

func mustExec(t *testing.T, db Execer, query string, args ...interface{}) {
	_, err := db.Exec(query, args...)
	require.NoError(t, err)
}
