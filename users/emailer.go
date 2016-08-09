package main

import (
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"net/url"

	"github.com/Sirupsen/logrus"
	"github.com/jordan-wright/email"
	"github.com/sendgrid/sendgrid-go"
)

var (
	errUnsupportedEmailProtocol = errors.New("Unsupported email protocol")
)

func loginURL(email, rawToken, domain string) string {
	return fmt.Sprintf(
		"%s/#/login/%s/%s",
		domain,
		url.QueryEscape(email),
		url.QueryEscape(rawToken),
	)
}

func inviteURL(email, rawToken, domain, orgName string) string {
	return fmt.Sprintf(
		"%s/#/login/%s/%s/%s",
		domain,
		orgName,
		url.QueryEscape(email),
		url.QueryEscape(rawToken),
	)
}

func organizationURL(domain, orgExternalID string) string {
	return fmt.Sprintf("%s/#/org/%s", domain, orgExternalID)
}

type emailer interface {
	LoginEmail(u *user, token string) error
	InviteEmail(inviter, invited *user, orgExternalID, orgName, token string) error
	GrantAccessEmail(inviter, invited *user, orgExternalID, orgName string) error
}

type smtpEmailer struct {
	templates   templateEngine
	sender      func(*email.Email) error
	domain      string
	fromAddress string
}

func mustNewEmailer(emailURI, sendgridAPIKey, fromAddress string, templates templateEngine, domain string) emailer {
	if (emailURI == "") == (sendgridAPIKey == "") {
		logrus.Fatal("Must provide one of -email-uri or -sendgrid-api-key")
	}
	if emailURI != "" {
		var sender func(*email.Email) error
		u, err := url.Parse(emailURI)
		if err != nil {
			logrus.Fatal(fmt.Errorf("Error parsing -email-uri: %s", err))
		}
		switch u.Scheme {
		case "smtp":
			sender, err = smtpEmailSender(u)
		case "log":
			sender = logEmailSender()
		default:
			err = errUnsupportedEmailProtocol
		}
		if err != nil {
			logrus.Fatal(err)
		}
		return smtpEmailer{
			templates:   templates,
			sender:      sender,
			domain:      domain,
			fromAddress: fromAddress,
		}
	}
	client := sendgrid.NewSendGridClientWithApiKey(sendgridAPIKey)
	return sendgridEmailer{
		templates:   templates,
		client:      client,
		domain:      domain,
		fromAddress: fromAddress,
	}
}

// Takes a uri of the form smtp://username:password@hostname:port
func smtpEmailSender(u *url.URL) (func(e *email.Email) error, error) {
	host, port, err := net.SplitHostPort(u.Host)
	if err != nil {
		return nil, fmt.Errorf("Error parsing email server uri: %s", err)
	}
	if port == "" {
		port = "587"
	}
	addr := fmt.Sprintf("%s:%s", host, port)
	var auth smtp.Auth
	if u.User != nil {
		password, _ := u.User.Password()
		auth = smtp.PlainAuth("", u.User.Username(), password, host)
	}

	return func(e *email.Email) error {
		return e.Send(addr, auth)
	}, nil
}

// Takes a uri of the form log://, and just logs all emails, instead of sending them.
func logEmailSender() func(e *email.Email) error {
	return func(e *email.Email) error {
		body := string(e.Text)
		if body == "" {
			body = string(e.HTML)
		}
		logrus.Infof("[Email] From: %q, To: %q, Subject: %q, Body:\n%s", e.From, e.To, e.Subject, body)
		return nil
	}
}

func (s smtpEmailer) LoginEmail(u *user, token string) error {
	e := email.NewEmail()
	e.From = s.fromAddress
	e.To = []string{u.Email}
	e.Subject = "Login to Weave Cloud"
	data := map[string]interface{}{
		"LoginURL": loginURL(u.Email, token, s.domain),
		"RootURL":  s.domain,
	}
	e.Text = s.templates.quietBytes("login_email.text", data)
	e.HTML = s.templates.quietBytes("login_email.html", data)
	return s.sender(e)
}

func (s smtpEmailer) InviteEmail(inviter, invited *user, orgExternalID, orgName, token string) error {
	e := email.NewEmail()
	e.From = s.fromAddress
	e.To = []string{invited.Email}
	e.Subject = "You've been invited to Weave Cloud"
	data := map[string]interface{}{
		"InviterName":      inviter.Email,
		"LoginURL":         inviteURL(invited.Email, token, s.domain, orgExternalID),
		"RootURL":          s.domain,
		"OrganizationName": orgName,
	}
	e.Text = s.templates.quietBytes("invite_email.text", data)
	e.HTML = s.templates.quietBytes("invite_email.html", data)
	return s.sender(e)
}

func (s smtpEmailer) GrantAccessEmail(inviter, invited *user, orgExternalID, orgName string) error {
	e := email.NewEmail()
	e.From = s.fromAddress
	e.To = []string{invited.Email}
	e.Subject = "Weave Cloud access granted to instance"
	data := map[string]interface{}{
		"InviterName":      inviter.Email,
		"OrganizationName": orgName,
		"OrganizationURL":  organizationURL(orgExternalID, orgName),
	}
	e.Text = s.templates.quietBytes("grant_access_email.text", data)
	e.HTML = s.templates.quietBytes("grant_access_email.html", data)
	return s.sender(e)
}

type sendgridEmailer struct {
	templates   templateEngine
	client      *sendgrid.SGClient
	domain      string
	fromAddress string
}

func (s sendgridEmailer) sendgridEmail() *sendgrid.SGMail {
	mail := sendgrid.NewMail()
	mail.SetFrom(s.fromAddress)
	return mail
}

func (s sendgridEmailer) LoginEmail(u *user, token string) error {
	mail := s.sendgridEmail()
	mail.AddTo(u.Email)
	mail.SetSubject("Login to Weave Cloud")
	data := map[string]interface{}{
		"LoginURL": loginURL(u.Email, token, s.domain),
		"RootURL":  s.domain,
	}
	mail.SetText(string(s.templates.quietBytes("login_email.text", data)))
	mail.SetHTML(string(s.templates.quietBytes("login_email.html", data)))
	return s.client.Send(mail)
}

func (s sendgridEmailer) InviteEmail(inviter, invited *user, orgExternalID, orgName, token string) error {
	mail := s.sendgridEmail()
	mail.AddTo(invited.Email)
	mail.SetSubject("You've been invited to Weave Cloud")
	data := map[string]interface{}{
		"InviterName":      inviter.Email,
		"LoginURL":         inviteURL(invited.Email, token, s.domain, orgExternalID),
		"RootURL":          s.domain,
		"OrganizationName": orgName,
	}
	mail.SetText(string(s.templates.quietBytes("invite_email.text", data)))
	mail.SetHTML(string(s.templates.quietBytes("invite_email.html", data)))
	return s.client.Send(mail)
}

// GrantAccessEmail sends an email granting access.
func (s sendgridEmailer) GrantAccessEmail(inviter, invited *user, orgExternalID, orgName string) error {
	mail := s.sendgridEmail()
	mail.AddTo(invited.Email)
	mail.SetSubject("Weave Cloud access granted to instance")
	data := map[string]interface{}{
		"InviterName":      inviter.Email,
		"OrganizationName": orgName,
		"OrganizationURL":  organizationURL(orgExternalID, orgName),
	}
	mail.SetText(string(s.templates.quietBytes("grant_access_email.text", data)))
	mail.SetHTML(string(s.templates.quietBytes("grant_access_email.html", data)))
	return s.client.Send(mail)
}
