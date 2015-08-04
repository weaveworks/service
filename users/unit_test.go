// +build !integration

package main

import (
	"net/http"
	"testing"

	"github.com/jordan-wright/email"
	"golang.org/x/crypto/bcrypt"
)

var sentEmails []*email.Email
var app http.Handler

func setup(t *testing.T) {
	domain = "example.com"
	passwordHashingCost = bcrypt.MinCost
	sentEmails = nil
	sendEmail = testEmailSender

	var directLogin = false

	setupLogging("debug")
	setupStorage("memory://")
	setupTemplates()
	setupSessions("Test-Session-Secret-Which-Is-64-Bytes-Long-aa1a166556cb719f531cd")

	app = handler(directLogin)
}

func cleanup(t *testing.T) {
}

func testEmailSender(e *email.Email) error {
	sentEmails = append(sentEmails, e)
	return nil
}
