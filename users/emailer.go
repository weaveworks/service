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

type emailer interface {
	LoginEmail(u *user, token string) error
	InviteEmail(u *user, orgName, token string) error
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
		"Token":    token,
	}
	e.Text = s.templates.quietBytes("login_email.text", data)
	e.HTML = s.templates.quietBytes("login_email.html", data)
	return s.sender(e)
}

func (s smtpEmailer) InviteEmail(u *user, orgName, token string) error {
	e := email.NewEmail()
	e.From = s.fromAddress
	e.To = []string{u.Email}
	e.Subject = "You've been invited to Weave Cloud"
	data := map[string]interface{}{
		"LoginURL":         inviteURL(u.Email, token, s.domain, orgName),
		"RootURL":          s.domain,
		"Token":            token,
		"OrganizationName": orgName,
	}
	e.Text = s.templates.quietBytes("invite_email.text", data)
	e.HTML = s.templates.quietBytes("invite_email.html", data)
	return s.sender(e)
}

type sendgridEmailer struct {
	client      *sendgrid.SGClient
	domain      string
	fromAddress string
}

const (
	loginEmailTemplate  = "ccf3f6e2-20f9-4c41-bd72-ca204c1c3f6f"
	inviteEmailTemplate = "00ceaa49-857f-4ce4-bc39-789b7f56c886"
)

func (s sendgridEmailer) sendgridEmail(templateID string) *sendgrid.SGMail {
	mail := sendgrid.NewMail()
	mail.SetFrom(s.fromAddress)
	mail.SetText(" ")
	mail.SetHTML(" ")
	mail.SetSubject(" ")
	mail.AddFilter("templates", "enable", "1")
	mail.AddFilter("templates", "template_id", templateID)
	return mail
}

func (s sendgridEmailer) LoginEmail(u *user, token string) error {
	mail := s.sendgridEmail(loginEmailTemplate)
	mail.AddTo(u.Email)
	mail.AddSubstitution(":login_url", loginURL(u.Email, token, s.domain))
	mail.AddSubstitution(":root_url", s.domain)
	return s.client.Send(mail)
}

func (s sendgridEmailer) InviteEmail(u *user, orgName, token string) error {
	mail := s.sendgridEmail(inviteEmailTemplate)
	mail.AddTo(u.Email)
	mail.AddSubstitution(":login_url", inviteURL(u.Email, token, s.domain, orgName))
	mail.AddSubstitution(":root_url", s.domain)
	mail.AddSubstitution(":org_name", orgName)
	return s.client.Send(mail)
}
