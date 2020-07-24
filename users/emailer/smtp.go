package emailer

import (
	"context"
	"fmt"
	"net"
	"net/smtp"
	"net/textproto"
	"net/url"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/jordan-wright/email"
	opentracing "github.com/opentracing/opentracing-go"
	otlog "github.com/opentracing/opentracing-go/log"
	sendgrid "github.com/sendgrid/sendgrid-go"
	"golang.org/x/time/rate"

	"github.com/weaveworks/service/billing-api/trial"
	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/templates"
	"github.com/weaveworks/service/users/weeklyreports"
)

// SMTPEmailer is an emailer which sends over SMTP. It is exposed for testing.
type SMTPEmailer struct {
	Templates      templates.Engine
	SendDirectly   func(context.Context, *email.Email) error
	SendGridClient *sendgrid.Client
	Domain         string
	FromAddress    string
}

func newSMTPEmailer(fromAddress string, templates templates.Engine, domain string, sendDirectly func(context.Context, *email.Email) error) Emailer {
	var sendGridClient *sendgrid.Client
	sendGridAPIKey, ok := os.LookupEnv("SENDGRID_API_KEY")
	if ok {
		sendGridClient = sendgrid.NewSendClient(sendGridAPIKey)
	}
	return SMTPEmailer{
		Templates:      templates,
		SendDirectly:   sendDirectly,
		SendGridClient: sendGridClient,
		Domain:         domain,
		FromAddress:    fromAddress,
	}
}

// Date format to use in email templates
const dateFormat = "January 2 2006"
const emailWrapperFilename = "wrapper.html"

// Takes a uri of the form smtp://username:password@hostname:port
func smtpEmailSender(u *url.URL) (func(ctx context.Context, e *email.Email) error, error) {
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
	// Allow up to 2 emails through without pause, then limit to one every 30 seconds.
	limiter := rate.NewLimiter(rate.Every(30*time.Second), 2)

	return func(ctx context.Context, e *email.Email) error {
		span, ctx := opentracing.StartSpanFromContext(ctx, "smtpSend")
		defer span.Finish()
		id := make(textproto.MIMEHeader)
		uid := uuid.New().String()
		id.Add("X-Entity-Ref-ID", uid)
		e.Headers = id
		if err := limiter.Wait(ctx); err != nil {
			return err
		}

		span.LogFields(otlog.String("sending now", e.Subject))
		return e.Send(addr, auth)
	}, nil
}

// WeeklyReportEmail sends the weekly report email
func (s SMTPEmailer) WeeklyReportEmail(ctx context.Context, members []*users.User, report *weeklyreports.Report) error {
	organizationURL := organizationURL(s.Domain, report.Organization.ExternalID)
	summary := weeklyreports.EmailSummaryFromReport(report, organizationURL)

	e := email.NewEmail()
	e.From = s.FromAddress
	e.To = collectEmails(members)
	e.Subject = fmt.Sprintf("%s · %s Report", summary.DateInterval, summary.Organization.Name)
	data := map[string]interface{}{
		"Report":         summary,
		"Unsubscribable": true,
	}
	e.Text = s.Templates.QuietBytes("weekly_report_email.text", data)
	e.HTML = s.Templates.EmbedHTML("weekly_report_email.html", emailWrapperFilename, "", data)
	return s.SendThroughSendGrid(ctx, e, weeklyReportGroupID)
}

// LoginEmail sends the login email
func (s SMTPEmailer) LoginEmail(ctx context.Context, u *users.User, token string, queryParams map[string]string) error {
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
	return s.SendDirectly(ctx, e)
}

// InviteToTeamEmail sends the invite email
func (s SMTPEmailer) InviteToTeamEmail(ctx context.Context, inviter, invited *users.User, teamExternalID, teamName, token string) error {
	e := email.NewEmail()
	e.From = s.FromAddress
	e.To = []string{invited.Email}
	e.Subject = "You've been invited to Weave Cloud"
	data := map[string]interface{}{
		"InviterName": inviter.Email,
		"LoginURL":    inviteToTeamURL(invited.Email, token, s.Domain, teamExternalID),
		"RootURL":     s.Domain,
		"TeamName":    teamName,
	}
	e.Text = s.Templates.QuietBytes("invite_to_team_email.text", data)
	e.HTML = s.Templates.EmbedHTML("invite_to_team_email.html", emailWrapperFilename, e.Subject, data)
	return s.SendDirectly(ctx, e)
}

// GrantAccessToTeamEmail sends the grant access email
func (s SMTPEmailer) GrantAccessToTeamEmail(ctx context.Context, inviter, invited *users.User, teamExternalID, teamName string) error {
	e := email.NewEmail()
	e.From = s.FromAddress
	e.To = []string{invited.Email}
	e.Subject = "Weave Cloud access granted to instance"
	data := map[string]interface{}{
		"InviterName": inviter.Email,
		"TeamName":    teamName,
		"TeamURL":     teamURL(s.Domain, teamExternalID),
	}
	e.Text = s.Templates.QuietBytes("grant_access_to_team_email.text", data)
	e.HTML = s.Templates.EmbedHTML("grant_access_to_team_email.html", emailWrapperFilename, e.Subject, data)
	return s.SendDirectly(ctx, e)
}

// TrialPendingExpiryEmail notifies all members of the organization that
// their trial is about to expire.
func (s SMTPEmailer) TrialPendingExpiryEmail(ctx context.Context, members []*users.User, orgExternalID, orgName string, trialExpiresAt time.Time) error {
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
	return s.SendDirectly(ctx, e)
}

// TrialExpiredEmail notifies all members of the organization that
// their trial has expired.
func (s SMTPEmailer) TrialExpiredEmail(ctx context.Context, members []*users.User, orgExternalID, orgName string) error {
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
	return s.SendDirectly(ctx, e)
}

// TrialExtendedEmail notifies all members of the organization that the trial
// period has been extended.
func (s SMTPEmailer) TrialExtendedEmail(ctx context.Context, members []*users.User, orgExternalID, orgName string, trialExpiresAt time.Time) error {
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
	return s.SendDirectly(ctx, e)
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
func (s SMTPEmailer) RefuseDataUploadEmail(ctx context.Context, members []*users.User, orgExternalID, orgName string) error {
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
	return s.SendDirectly(ctx, e)
}
