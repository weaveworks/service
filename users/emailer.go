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

const (
	fromAddress = "Scope Support <support@weave.works>"
	domain      = "scope.weave.works"
	rootURL     = "https://" + domain
)

var (
	errUnsupportedEmailProtocol = errors.New("Unsupported email protocol")
)

type emailer interface {
	WelcomeEmail(u *user) error
	ApprovedEmail(u *user, token string) error
	LoginEmail(u *user, token string) error
	InviteEmail(u *user, token string) error
}

type emailSender func(*email.Email) error

type smtpEmailer struct {
	templates templateEngine
	sender    emailSender
}

func makeSMTPEmailer(sender emailSender, templates templateEngine) emailer {
	return smtpEmailer{templates, sender}
}

func (s smtpEmailer) WelcomeEmail(u *user) error {
	return s.sender(welcomeEmail(s.templates, u))
}

func (s smtpEmailer) ApprovedEmail(u *user, token string) error {
	return s.sender(approvedEmail(s.templates, u, token))
}

func (s smtpEmailer) LoginEmail(u *user, token string) error {
	return s.sender(loginEmail(s.templates, u, token))
}

func (s smtpEmailer) InviteEmail(u *user, token string) error {
	return s.sender(inviteEmail(s.templates, u, token))
}

func mustNewEmailSender(emailURI string) emailSender {
	m, err := smtpEmailSender(emailURI)
	if err != nil {
		logrus.Fatal(err)
	}
	return m
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

func welcomeEmail(t templateEngine, u *user) *email.Email {
	e := email.NewEmail()
	e.From = fromAddress
	e.To = []string{u.Email}
	e.Subject = "Welcome to Scope"
	e.Text = t.quietBytes("welcome_email.text", nil)
	e.HTML = t.quietBytes("welcome_email.html", nil)
	return e
}

func approvedEmail(t templateEngine, u *user, token string) *email.Email {
	e := email.NewEmail()
	e.From = fromAddress
	e.To = []string{u.Email}
	e.Subject = "Scope account approved"
	data := map[string]interface{}{
		"LoginURL":   loginURL(u.Email, token),
		"RootURL":    rootURL,
		"Token":      token,
		"ProbeToken": u.Organization.ProbeToken,
	}
	e.Text = t.quietBytes("approved_email.text", data)
	e.HTML = t.quietBytes("approved_email.html", data)
	return e
}

func loginEmail(t templateEngine, u *user, token string) *email.Email {
	e := email.NewEmail()
	e.From = fromAddress
	e.To = []string{u.Email}
	e.Subject = "Login to Scope"
	data := map[string]interface{}{
		"LoginURL": loginURL(u.Email, token),
		"RootURL":  rootURL,
		"Token":    token,
	}
	e.Text = t.quietBytes("login_email.text", data)
	e.HTML = t.quietBytes("login_email.html", data)
	return e
}

func loginURL(email, rawToken string) string {
	return fmt.Sprintf(
		"https://%s/#/login/%s/%s",
		domain,
		url.QueryEscape(email),
		url.QueryEscape(rawToken),
	)
}

func inviteEmail(t templateEngine, u *user, token string) *email.Email {
	e := email.NewEmail()
	e.From = fromAddress
	e.To = []string{u.Email}
	e.Subject = "You've been invited to Scope"
	data := map[string]interface{}{
		"LoginURL":         loginURL(u.Email, token),
		"RootURL":          rootURL,
		"Token":            token,
		"OrganizationName": u.Organization.Name,
	}
	e.Text = t.quietBytes("invite_email.text", data)
	e.HTML = t.quietBytes("invite_email.html", data)
	return e
}

type sendgridEmailer struct {
	client *sendgrid.SGClient
}

func makeSendgridEmailer(apikey string) emailer {
	client := sendgrid.NewSendGridClientWithApiKey(apikey)
	return sendgridEmailer{client}
}

const (
	welcomeEmailTemplate  = "1721d506-dcf7-4e84-a629-63d34a86325b"
	approvedEmailTemplate = "980cc9b5-6872-4596-8560-5c220a9341fd"
	loginEmailTemplate    = "ccf3f6e2-20f9-4c41-bd72-ca204c1c3f6f"
	inviteEmailTemplate   = "00ceaa49-857f-4ce4-bc39-789b7f56c886"
)

func (s sendgridEmailer) WelcomeEmail(u *user) error {
	mail := sendgrid.NewMail()
	mail.AddFilter("template", "enable", "1")
	mail.AddFilter("template", "template_id", welcomeEmailTemplate)
	mail.AddTo(u.Email)
	return s.client.Send(mail)
}

func (s sendgridEmailer) ApprovedEmail(u *user, token string) error {
	mail := sendgrid.NewMail()
	mail.AddFilter("template", "enable", "1")
	mail.AddFilter("template", "template_id", approvedEmailTemplate)
	mail.AddTo(u.Email)
	mail.AddSubstitution(":login_url", loginURL(u.Email, token))
	mail.AddSubstitution(":root_url", rootURL)
	return s.client.Send(mail)
}

func (s sendgridEmailer) LoginEmail(u *user, token string) error {
	mail := sendgrid.NewMail()
	mail.AddFilter("template", "enable", "1")
	mail.AddFilter("template", "template_id", loginEmailTemplate)
	mail.AddTo(u.Email)
	mail.AddSubstitution(":login_url", loginURL(u.Email, token))
	mail.AddSubstitution(":root_url", rootURL)
	return s.client.Send(mail)
}

func (s sendgridEmailer) InviteEmail(u *user, token string) error {
	mail := sendgrid.NewMail()
	mail.AddFilter("template", "enable", "1")
	mail.AddFilter("template", "template_id", inviteEmailTemplate)
	mail.AddTo(u.Email)
	mail.AddSubstitution(":login_url", loginURL(u.Email, token))
	mail.AddSubstitution(":root_url", rootURL)
	mail.AddSubstitution(":org_name", u.Organization.Name)
	return s.client.Send(mail)
}
