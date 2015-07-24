// +build !integration

package main

import (
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/jordan-wright/email"
)

var sentEmails []*email.Email

func setup(t *testing.T) {
	domain = "example.com"
	passwordHashingCost = bcrypt.MinCost
	sentEmails = nil
	sendEmail = testEmailSender

	setupLogging("debug")
	setupTemplates()
	setupSessions("Test-Session-Secret-Which-Is-64-Bytes-Long-aa1a166556cb719f531cd")
	setupStorage()
}

func cleanup(t *testing.T) {
}

func testEmailSender(e *email.Email) error {
	sentEmails = append(sentEmails, e)
	return nil
}
