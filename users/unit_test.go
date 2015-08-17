// +build !integration

package main

import (
	"testing"

	"github.com/jordan-wright/email"
	"golang.org/x/crypto/bcrypt"
)

var (
	sentEmails []*email.Email
	app        *api
	storage    database
	sessions   sessionStore
)

func setup(t *testing.T) {
	passwordHashingCost = bcrypt.MinCost

	var directLogin = false

	setupLogging("debug")
	storage = mustNewDatabase("memory://")
	sessions = mustNewSessionStore("Test-Session-Secret-Which-Is-64-Bytes-Long-aa1a166556cb719f531cd", storage)
	templates := mustNewTemplateEngine()

	sentEmails = nil
	app = newAPI(directLogin, testEmailSender, sessions, storage, templates)
}

func cleanup(t *testing.T) {
}

func testEmailSender(e *email.Email) error {
	sentEmails = append(sentEmails, e)
	return nil
}
