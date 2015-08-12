package main

import (
	"errors"
	"fmt"
	htmlTemplate "html/template"
	"net"
	"net/smtp"
	"net/url"

	"github.com/Sirupsen/logrus"
	"github.com/jordan-wright/email"
)

const (
	fromAddress = "Scope Support <support@weave.works>"
	domain      = "run.weave.works"
)

var (
	sendEmail                   func(*email.Email) error
	errUnsupportedEmailProtocol = errors.New("Unsupported email protocol")
)

func setupEmail(emailURI string) {
	var err error
	sendEmail, err = smtpEmailSender(emailURI)
	if err != nil {
		logrus.Fatal(err)
	}
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

func sendWelcomeEmail(u *user) error {
	e := email.NewEmail()
	e.From = fromAddress
	e.Bcc = []string{fromAddress}
	e.To = []string{u.Email}
	e.Subject = "Welcome to Scope"
	e.Text = quietTemplateBytes("welcome_email.text", nil)
	e.HTML = quietTemplateBytes("welcome_email.html", nil)
	return sendEmail(e)
}

func sendApprovedEmail(u *user, token string) error {
	e := email.NewEmail()
	e.From = fromAddress
	e.To = []string{u.Email}
	e.Subject = "Scope account approved"
	data := map[string]interface{}{
		"LoginURL":   loginURL(u.Email, token),
		"LoginLink":  loginLink(u.Email, token),
		"Token":      token,
		"ProbeToken": u.Organization.ProbeToken,
	}
	e.Text = quietTemplateBytes("approved_email.text", data)
	e.HTML = quietTemplateBytes("approved_email.html", data)
	return sendEmail(e)
}

func sendLoginEmail(u *user, token string) error {
	e := email.NewEmail()
	e.From = fromAddress
	e.To = []string{u.Email}
	e.Subject = "Login to Scope"
	data := map[string]interface{}{
		"LoginURL":   loginURL(u.Email, token),
		"LoginLink":  loginLink(u.Email, token),
		"Token":      token,
		"ProbeToken": u.Organization.ProbeToken,
	}
	e.Text = quietTemplateBytes("login_email.text", data)
	e.HTML = quietTemplateBytes("login_email.html", data)
	return sendEmail(e)
}

func loginURL(email, rawToken string) string {
	return fmt.Sprintf(
		"http://%s/#/login/%s/%s",
		domain,
		url.QueryEscape(email),
		url.QueryEscape(rawToken),
	)
}

func loginLink(email, rawToken string) htmlTemplate.HTML {
	url := loginURL(email, rawToken)
	return htmlTemplate.HTML(
		fmt.Sprintf(
			"<a href=\"%s\">%s</a>",
			url,
			htmlTemplate.HTMLEscapeString(url),
		),
	)
}

func sendInviteEmail(u *user, token string) error {
	e := email.NewEmail()
	e.From = fromAddress
	e.To = []string{u.Email}
	e.Subject = "You've been invited to Scope"
	data := map[string]interface{}{
		"LoginURL":         loginURL(u.Email, token),
		"LoginLink":        loginLink(u.Email, token),
		"Token":            token,
		"OrganizationName": u.Organization.Name,
		"ProbeToken":       u.Organization.ProbeToken,
	}
	e.Text = quietTemplateBytes("invite_email.text", data)
	e.HTML = quietTemplateBytes("invite_email.html", data)
	return sendEmail(e)
}
