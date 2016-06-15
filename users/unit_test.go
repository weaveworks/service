// +build !integration

package main

import (
	"testing"

	"github.com/jordan-wright/email"
	"golang.org/x/crypto/bcrypt"

	"github.com/weaveworks/service/users/login"
)

var (
	sentEmails []*email.Email
	app        *api
	storage    database
	logins     *login.Providers
	sessions   sessionStore
	domain     = "http://fake.scope"
)

func setup(t *testing.T) {
	passwordHashingCost = bcrypt.MinCost

	var directLogin = false

	setupLogging("debug")
	storage = mustNewDatabase("memory://", "")
	sessions = mustNewSessionStore("Test-Session-Secret-Which-Is-64-Bytes-Long-aa1a166556cb719f531cd", storage)
	templates := mustNewTemplateEngine()
	logins = login.NewProviders()

	sentEmails = nil
	emailer := smtpEmailer{templates, testEmailSender, domain, "test@test.com"}
	app = newAPI(directLogin, emailer, sessions, storage, logins, templates)
}

func cleanup(t *testing.T) {
	logins.Reset()
}

func testEmailSender(e *email.Email) error {
	sentEmails = append(sentEmails, e)
	return nil
}
