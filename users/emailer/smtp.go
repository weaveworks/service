package emailer

import (
	"fmt"
	"net"
	"net/smtp"
	"net/textproto"
	"net/url"
	"time"

	"github.com/google/uuid"
	"github.com/jordan-wright/email"
	"github.com/weaveworks/service/billing-api/trial"
	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/templates"
)

// SMTPEmailer is an emailer which sends over SMTP. It is exposed for testing.
// Implements Emailer, see MustNew() for instantiation.
type SMTPEmailer struct {
	Templates   templates.Engine
	Sender      func(*email.Email) error
	Domain      string
	FromAddress string
}

// Date format to use in email templates
const dateFormat = "January 2 2006"
const emailWrapperFilename = "wrapper.html"

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
		id := make(textproto.MIMEHeader)
		uid := uuid.New().String()
		id.Add("X-Entity-Ref-ID", uid)
		e.Headers = id

		return e.Send(addr, auth)
	}, nil
}

// LoginEmail sends the login email
func (s SMTPEmailer) LoginEmail(u *users.User, token string, queryParams map[string]string) error {
	e := email.NewEmail()
	e.From = s.FromAddress
	e.To = []string{u.Email}
	e.Subject = "Login to Weave Cloud"
	data := map[string]interface{}{
		"LoginURL": loginURL(u.Email, token, s.Domain, queryParams),
		"RootURL":  s.Domain,
	}
	e.Text = s.Templates.QuietBytes("login_email.text", data)
	e.HTML = s.Templates.EmbedHTML("login_email.html", emailWrapperFilename, e.Subject, data)
	return s.Sender(e)
}

// InviteEmail sends the invite email
func (s SMTPEmailer) InviteEmail(inviter, invited *users.User, orgExternalID, orgName, token string) error {
	e := email.NewEmail()
	e.From = s.FromAddress
	e.To = []string{invited.Email}
	e.Subject = "You've been invited to Weave Cloud"
	data := map[string]interface{}{
		"InviterName":      inviter.Email,
		"LoginURL":         inviteURL(invited.Email, token, s.Domain, orgExternalID),
		"RootURL":          s.Domain,
		"OrganizationName": orgName,
	}
	e.Text = s.Templates.QuietBytes("invite_email.text", data)
	e.HTML = s.Templates.EmbedHTML("invite_email.html", emailWrapperFilename, e.Subject, data)
	return s.Sender(e)
}

// GrantAccessEmail sends the grant access email
func (s SMTPEmailer) GrantAccessEmail(inviter, invited *users.User, orgExternalID, orgName string) error {
	e := email.NewEmail()
	e.From = s.FromAddress
	e.To = []string{invited.Email}
	e.Subject = "Weave Cloud access granted to instance"
	data := map[string]interface{}{
		"InviterName":      inviter.Email,
		"OrganizationName": orgName,
		"OrganizationURL":  organizationURL(s.Domain, orgExternalID),
	}
	e.Text = s.Templates.QuietBytes("grant_access_email.text", data)
	e.HTML = s.Templates.EmbedHTML("grant_access_email.html", emailWrapperFilename, e.Subject, data)
	return s.Sender(e)
}

// TrialPendingExpiryEmail notifies all members of the organization that
// their trial is about to expire.
func (s SMTPEmailer) TrialPendingExpiryEmail(members []*users.User, orgExternalID, orgName string, trialExpiresAt time.Time) error {
	e := email.NewEmail()
	e.From = s.FromAddress
	e.To = collectEmails(members)
	e.Subject = "Your Weave Cloud trial expires soon!"
	data := map[string]interface{}{
		"OrganizationName": orgName,
		"BillingURL":       billingURL(s.Domain, orgExternalID),
		"TrialExpiresAt":   trialExpiresAt.Format(dateFormat),
		"TrialLeft":        trialLeft(trialExpiresAt),
	}
	e.Text = s.Templates.QuietBytes("trial_pending_expiry_email.text", data)
	e.HTML = s.Templates.EmbedHTML("trial_pending_expiry_email.html", emailWrapperFilename, e.Subject, data)

	return s.Sender(e)
}

// TrialExpiredEmail notifies all members of the organization that
// their trial has expired.
func (s SMTPEmailer) TrialExpiredEmail(members []*users.User, orgExternalID, orgName string) error {
	e := email.NewEmail()
	e.From = s.FromAddress
	e.To = collectEmails(members)
	e.Subject = "Your Weave Cloud trial expired"
	data := map[string]interface{}{
		"OrganizationName": orgName,
		"BillingURL":       billingURL(s.Domain, orgExternalID),
	}
	e.Text = s.Templates.QuietBytes("trial_expired_email.text", data)
	e.HTML = s.Templates.EmbedHTML("trial_expired_email.html", emailWrapperFilename, e.Subject, data)

	return s.Sender(e)
}

// TrialExtendedEmail notifies all members of the organization that the trial
// period has been extended.
func (s SMTPEmailer) TrialExtendedEmail(members []*users.User, orgExternalID, orgName string, trialExpiresAt time.Time) error {
	left := trialLeft(trialExpiresAt)
	data := map[string]interface{}{
		"OrganizationName": orgName,
		"BillingURL":       billingURL(s.Domain, orgExternalID),
		"TrialExpiresAt":   trialExpiresAt.Format(dateFormat),
		"TrialLeft":        left,
	}
	e := email.NewEmail()
	e.From = s.FromAddress
	e.To = collectEmails(members)
	e.Subject = fmt.Sprintf("%s left of your free trial", left)
	e.Text = s.Templates.QuietBytes("trial_extended_email.text", data)
	e.HTML = s.Templates.EmbedHTML("trial_extended_email.html", emailWrapperFilename, e.Subject, data)

	return s.Sender(e)
}

func trialLeft(expires time.Time) string {
	days := trial.Remaining(expires, time.Now().UTC())
	if days == 1 {
		return "1 day"
	}
	return fmt.Sprintf("%d days", days)
}

// RefuseDataUploadEmail notifies all members of the organization that the trial
// period has been expired for a while and we now block their data upload.
func (s SMTPEmailer) RefuseDataUploadEmail(members []*users.User, orgExternalID, orgName string) error {
	data := map[string]interface{}{
		"OrganizationName": orgName,
		"BillingURL":       billingURL(s.Domain, orgExternalID),
	}
	e := email.NewEmail()
	e.From = s.FromAddress
	e.To = collectEmails(members)
	e.Subject = "Sorry to see you leave Weave Cloud!"
	e.Text = s.Templates.QuietBytes("refuse_data_upload_email.text", data)
	e.HTML = s.Templates.EmbedHTML("refuse_data_upload_email.html", emailWrapperFilename, e.Subject, data)

	return s.Sender(e)
}
