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

	users = make(map[string]*User)

	if err := loadTemplates(); err != nil {
		t.Fatal(err)
	}
}

func cleanup(t *testing.T) {
}

func testEmailSender(e *email.Email) error {
	sentEmails = append(sentEmails, e)
	return nil
}
