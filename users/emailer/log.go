package emailer

import (
	log "github.com/Sirupsen/logrus"
	"github.com/jordan-wright/email"
)

// Takes a uri of the form log://, and just logs all emails, instead of sending them.
func logEmailSender() func(e *email.Email) error {
	return func(e *email.Email) error {
		body := string(e.Text)
		if body == "" {
			body = string(e.HTML)
		}
		log.Infof("[Email] From: %q, To: %q, Subject: %q, Body:\n%s", e.From, e.To, e.Subject, body)
		return nil
	}
}
