package main

import (
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"net/url"

	"github.com/jordan-wright/email"
	"github.com/sendgrid/sendgrid-go"
)

const (
	fromAddress = "Scope Support <support@weave.works>"
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

type emailer interface {
	WelcomeEmail(u *user) error
	ApprovedEmail(u *user, token string) error
	LoginEmail(u *user, token string) error
	InviteEmail(u *user, token string) error
}

type smtpEmailer struct {
	templates templateEngine
	sender    func(*email.Email) error
	domain    string
}

func makeEmailer(emailURI, sendgridAPIKey string, templates templateEngine, domain string) (emailer, error) {
	if (emailURI == "") == (sendgridAPIKey == "") {
		return nil, fmt.Errorf("Must provide one of -email-uri or -sendgrid-api-key")
	}
	if emailURI != "" {
		sender, err := smtpEmailSender(emailURI)
		if err != nil {
			return nil, err
		}
		return smtpEmailer{templates, sender, domain}, nil
	}
	client := sendgrid.NewSendGridClientWithApiKey(sendgridAPIKey)
	return sendgridEmailer{client, domain}, nil
}

// Takes a uri of the form smtp://username:password@hostname:port
func smtpEmailSender(uri string) (func(e *email.Email) error, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("Error parsing email server uri: %s", err)
	}
	if u.Scheme != "smtp" {
		return nil, errUnsupportedEmailProtocol
	}

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

func (s smtpEmailer) WelcomeEmail(u *user) error {
	e := email.NewEmail()
	e.From = fromAddress
	e.To = []string{u.Email}
	e.Subject = "Welcome to Scope"
	e.Text = s.templates.quietBytes("welcome_email.text", nil)
	e.HTML = s.templates.quietBytes("welcome_email.html", nil)
	return s.sender(e)
}

func (s smtpEmailer) ApprovedEmail(u *user, token string) error {
	e := email.NewEmail()
	e.From = fromAddress
	e.To = []string{u.Email}
	e.Subject = "Scope account approved"
	data := map[string]interface{}{
		"LoginURL":   loginURL(u.Email, token, s.domain),
		"RootURL":    s.domain,
		"Token":      token,
		"ProbeToken": u.Organization.ProbeToken,
	}
	e.Text = s.templates.quietBytes("approved_email.text", data)
	e.HTML = s.templates.quietBytes("approved_email.html", data)
	return s.sender(e)
}

func (s smtpEmailer) LoginEmail(u *user, token string) error {
	e := email.NewEmail()
	e.From = fromAddress
	e.To = []string{u.Email}
	e.Subject = "Login to Scope"
	data := map[string]interface{}{
		"LoginURL": loginURL(u.Email, token, s.domain),
		"RootURL":  s.domain,
		"Token":    token,
	}
	e.Text = s.templates.quietBytes("login_email.text", data)
	e.HTML = s.templates.quietBytes("login_email.html", data)
	return s.sender(e)
}

func (s smtpEmailer) InviteEmail(u *user, token string) error {
	e := email.NewEmail()
	e.From = fromAddress
	e.To = []string{u.Email}
	e.Subject = "You've been invited to Scope"
	data := map[string]interface{}{
		"LoginURL":         loginURL(u.Email, token, s.domain),
		"RootURL":          s.domain,
		"Token":            token,
		"OrganizationName": u.Organization.Name,
	}
	e.Text = s.templates.quietBytes("invite_email.text", data)
	e.HTML = s.templates.quietBytes("invite_email.html", data)
	return s.sender(e)
}

type sendgridEmailer struct {
	client *sendgrid.SGClient
	domain string
}

const (
	welcomeEmailTemplate  = "1721d506-dcf7-4e84-a629-63d34a86325b"
	approvedEmailTemplate = "980cc9b5-6872-4596-8560-5c220a9341fd"
	loginEmailTemplate    = "ccf3f6e2-20f9-4c41-bd72-ca204c1c3f6f"
	inviteEmailTemplate   = "00ceaa49-857f-4ce4-bc39-789b7f56c886"
)

func sendgridEmail(templateID string) *sendgrid.SGMail {
	mail := sendgrid.NewMail()
	mail.SetFrom(fromAddress)
	mail.SetText(" ")
	mail.SetHTML(" ")
	mail.SetSubject(" ")
	mail.AddFilter("templates", "enable", "1")
	mail.AddFilter("templates", "template_id", templateID)
	return mail
}

func (s sendgridEmailer) WelcomeEmail(u *user) error {
	mail := sendgridEmail(welcomeEmailTemplate)
	mail.AddTo(u.Email)
	return s.client.Send(mail)
}

func (s sendgridEmailer) ApprovedEmail(u *user, token string) error {
	mail := sendgridEmail(approvedEmailTemplate)
	mail.AddTo(u.Email)
	mail.AddSubstitution(":login_url", loginURL(u.Email, token, s.domain))
	mail.AddSubstitution(":root_url", s.domain)
	return s.client.Send(mail)
}

func (s sendgridEmailer) LoginEmail(u *user, token string) error {
	mail := sendgridEmail(loginEmailTemplate)
	mail.AddTo(u.Email)
	mail.AddSubstitution(":login_url", loginURL(u.Email, token, s.domain))
	mail.AddSubstitution(":root_url", s.domain)
	return s.client.Send(mail)
}

func (s sendgridEmailer) InviteEmail(u *user, token string) error {
	mail := sendgridEmail(inviteEmailTemplate)
	mail.AddTo(u.Email)
	mail.AddSubstitution(":login_url", loginURL(u.Email, token, s.domain))
	mail.AddSubstitution(":root_url", s.domain)
	mail.AddSubstitution(":org_name", u.Organization.Name)
	return s.client.Send(mail)
}
