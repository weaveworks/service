package main

import (
	"net/smtp"

	"github.com/jordan-wright/email"
)

const (
	fromAddress = "Weave Support <support@weave.works>"
)

var sendEmail func(*email.Email) error

func stubEmailSender(e *email.Email) error {
	return nil
}

func smtpEmailSender(addr string, auth smtp.Auth) func(e *email.Email) error {
	return func(e *email.Email) error {
		return e.Send(addr, auth)
	}
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

func SendLoginEmail(u *User) error {
	u.Token = randomString()
	e := email.NewEmail()
	e.From = fromAddress
	e.To = []string{u.Email}
	e.Subject = "Login to Weave"
	e.Text = quietTemplateBytes("login_email.text", u)
	e.HTML = quietTemplateBytes("login_email.html", u)
	return sendEmail(e)
}
