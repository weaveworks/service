// +build !integration

package main

import (
	"testing"

	"github.com/jordan-wright/email"
)

var sentEmails []*email.Email

func setup(t *testing.T) {
	domain = "example.com"
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
