package emailer

import (
	"context"

	"github.com/jordan-wright/email"
	log "github.com/sirupsen/logrus"
)

// logEmailSender just logs all emails, instead of sending them.
func logEmailSender() func(ctx context.Context, e *email.Email) error {
	return func(ctx context.Context, e *email.Email) error {
		body := string(e.Text)
		if body == "" {
			body = string(e.HTML)
		}
		log.Infof("[Email] From: %q, To: %q, Subject: %q, Body:\n%s", e.From, e.To, e.Subject, body)
		return nil
	}
}
