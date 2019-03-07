package emailer

import (
	netmail "net/mail"

	"github.com/jordan-wright/email"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
)

// ToSendGridMessage converts our standard email data structure into one that SendGrid accepts.
func ToSendGridMessage(e *email.Email) *mail.SGMailV3 {
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

	// Finally, fill in the email content.
	message.AddContent(
		mail.NewContent("text/plain", string(e.Text)),
		mail.NewContent("text/html", string(e.HTML)),
	)
	return message
}

// SendThroughSendGrid sends an email through SendGrid if possible, otherwise sends directly.
func (s SMTPEmailer) SendThroughSendGrid(e *email.Email) error {
	if s.SendGridClient != nil {
		message := ToSendGridMessage(e)
		_, err := s.SendGridClient.Send(message)
		return err
	}
	return s.SendDirectly(e)
}
