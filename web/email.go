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
	fromAddress = "Weave Support <support@weave.works>"
)

var (
	sendEmail                   func(*email.Email) error
	ErrUnsupportedEmailProtocol = errors.New("Unsupported email protocol")
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
		return nil, ErrUnsupportedEmailProtocol
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

func SendWelcomeEmail(u *User) error {
	e := email.NewEmail()
	e.From = fromAddress
	e.To = []string{u.Email}
	e.Subject = "Welcome to Weave"
	e.Text = quietTemplateBytes("welcome_email.text", nil)
	e.HTML = quietTemplateBytes("welcome_email.html", nil)
	return sendEmail(e)
}

func SendLoginEmail(u *User, token string) error {
	e := email.NewEmail()
	e.From = fromAddress
	e.To = []string{u.Email}
	e.Subject = "Login to Weave"
	data := map[string]interface{}{
		"LoginURL":  u.LoginURL(token),
		"LoginLink": u.LoginLink(token),
		"Token":     token,
	}
	e.Text = quietTemplateBytes("login_email.text", data)
	e.HTML = quietTemplateBytes("login_email.html", data)
	return sendEmail(e)
}
