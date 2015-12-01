package main

import (
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"net/url"

	"github.com/Sirupsen/logrus"
	"github.com/jordan-wright/email"
)

const (
	fromAddress = "Scope Support <support@weave.works>"
	domain      = "scope.weave.works"
	rootURL     = "http://" + domain
)

var (
	errUnsupportedEmailProtocol = errors.New("Unsupported email protocol")
)

type emailSender func(*email.Email) error

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
		"http://%s/#/login/%s/%s",
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
