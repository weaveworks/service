package emailer

import (
	"github.com/sendgrid/sendgrid-go"

	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/templates"
)

type sendgridEmailer struct {
	templates   templates.Engine
	client      *sendgrid.SGClient
	domain      string
	fromAddress string
}

func (s sendgridEmailer) sendgridEmail() *sendgrid.SGMail {
	mail := sendgrid.NewMail()
	mail.SetFrom(s.fromAddress)
	return mail
}

func (s sendgridEmailer) LoginEmail(u *users.User, token string) error {
	mail := s.sendgridEmail()
	mail.AddTo(u.Email)
	mail.SetSubject("Login to Weave Cloud")
	data := map[string]interface{}{
		"LoginURL": loginURL(u.Email, token, s.domain),
		"RootURL":  s.domain,
	}
	mail.SetText(string(s.templates.QuietBytes("login_email.text", data)))
	mail.SetHTML(string(s.templates.QuietBytes("login_email.html", data)))
	return s.client.Send(mail)
}

func (s sendgridEmailer) InviteEmail(inviter, invited *users.User, orgExternalID, orgName, token string) error {
	mail := s.sendgridEmail()
	mail.AddTo(invited.Email)
	mail.SetSubject("You've been invited to Weave Cloud")
	data := map[string]interface{}{
		"InviterName":      inviter.Email,
		"LoginURL":         inviteURL(invited.Email, token, s.domain, orgExternalID),
		"RootURL":          s.domain,
		"OrganizationName": orgName,
	}
	mail.SetText(string(s.templates.QuietBytes("invite_email.text", data)))
	mail.SetHTML(string(s.templates.QuietBytes("invite_email.html", data)))
	return s.client.Send(mail)
}

// GrantAccessEmail sends an email granting access.
func (s sendgridEmailer) GrantAccessEmail(inviter, invited *users.User, orgExternalID, orgName string) error {
	mail := s.sendgridEmail()
	mail.AddTo(invited.Email)
	mail.SetSubject("Weave Cloud access granted to instance")
	data := map[string]interface{}{
		"InviterName":      inviter.Email,
		"OrganizationName": orgName,
		"OrganizationURL":  organizationURL(s.domain, orgExternalID),
	}
	mail.SetText(string(s.templates.QuietBytes("grant_access_email.text", data)))
	mail.SetHTML(string(s.templates.QuietBytes("grant_access_email.html", data)))
	return s.client.Send(mail)
}
