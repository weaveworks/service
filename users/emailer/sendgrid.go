package emailer

import (
	"context"
	netmail "net/mail"

	"github.com/jordan-wright/email"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
)

// See https://app.sendgrid.com/suppressions/advanced_suppression_manager
const weeklyReportGroupID = 9852

// ToSendGridMessage converts our standard email data structure into one that SendGrid accepts.
func ToSendGridMessage(e *email.Email, groupID int) *mail.SGMailV3 {
	message := new(mail.SGMailV3)
	message.Subject = e.Subject

	// Fill in the sender data.
	from, _ := netmail.ParseAddress(e.From)
	message.SetFrom(mail.NewEmail(from.Name, from.Address))

	// Add all the email receivers.
	personalization := mail.NewPersonalization()
	for _, email := range e.To {
		personalization.AddTos(mail.NewEmail("", email))
	}
	message.AddPersonalizations(personalization)

	// Classify the email in the appropriate unsubscribe group.
	asm := mail.NewASM()
	asm.SetGroupID(groupID)
	message.SetASM(asm)

	// Finally, fill in the email content.
	message.AddContent(
		mail.NewContent("text/plain", string(e.Text)),
		mail.NewContent("text/html", string(e.HTML)),
	)
	return message
}

// SendThroughSendGrid sends an email through SendGrid if possible, otherwise sends directly.
func (s SMTPEmailer) SendThroughSendGrid(ctx context.Context, e *email.Email, groupID int) error {
	if s.SendGridClient != nil {
		span, _ := opentracing.StartSpanFromContext(ctx, "SendThroughSendGrid")
		defer span.Finish()
		message := ToSendGridMessage(e, groupID)
		_, err := s.SendGridClient.Send(message)
		return err
	}
	return s.SendDirectly(ctx, e)
}
