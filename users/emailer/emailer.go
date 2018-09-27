package emailer

import (
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/jordan-wright/email"
	log "github.com/sirupsen/logrus"

	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/templates"
	"github.com/weaveworks/service/users/weekly-summary"
)

// ErrUnsupportedEmailProtocol is the error when an email protocol is unsupported.
var ErrUnsupportedEmailProtocol = errors.New("unsupported email protocol")

// Emailer is the interface which emailers implement. There should be a method
// for each type of email we send.
type Emailer interface {
	LoginEmail(u *users.User, token string, queryParams map[string]string) error
	InviteEmail(inviter, invited *users.User, orgExternalID, orgName, token string) error
	GrantAccessEmail(inviter, invited *users.User, orgExternalID, orgName string) error
	TrialExtendedEmail(members []*users.User, orgExternalID, orgName string, expiresAt time.Time) error
	TrialPendingExpiryEmail(members []*users.User, orgExternalID, orgName string, expiresAt time.Time) error
	TrialExpiredEmail(members []*users.User, orgExternalID, orgName string) error
	RefuseDataUploadEmail(members []*users.User, orgExternalID, orgName string) error
	WeeklySummaryEmail(u *users.User, orgExternalID, orgName string, weeklyReport *weeklysummary.Report) error
}

// MustNew creates a new Emailer, from the URI, or panics.
// Supports scheme smtp:// and log:// in `emailURI`.
func MustNew(emailURI, fromAddress string, templates templates.Engine, domain string) Emailer {
	if emailURI == "" {
		log.Fatal("Must -email-uri")
	}
	var sender func(*email.Email) error
	u, err := url.Parse(emailURI)
	if err != nil {
		log.Fatalf("Error parsing -email-uri: %s", err)
	}
	switch u.Scheme {
	case "smtp":
		sender, err = smtpEmailSender(u)
	case "log":
		sender = logEmailSender()
	default:
		err = ErrUnsupportedEmailProtocol
	}
	if err != nil {
		log.Fatal(err)
	}
	return SMTPEmailer{
		Templates:   templates,
		Sender:      sender,
		Domain:      domain,
		FromAddress: fromAddress,
	}
}

func loginURL(email, rawToken, domain string, queryParams map[string]string) string {
	out, _ := url.ParseRequestURI(fmt.Sprintf(
		"%s/login/%s/%s",
		domain,
		url.PathEscape(email),
		url.PathEscape(rawToken),
	))
	q := out.Query()
	for key, value := range queryParams {
		q.Set(key, value)
	}
	out.RawQuery = q.Encode()
	return out.String()
}

func inviteURL(email, rawToken, domain, orgName string) string {
	return fmt.Sprintf(
		"%s/login/%s/%s/%s",
		domain,
		orgName,
		url.QueryEscape(email),
		url.QueryEscape(rawToken),
	)
}

func organizationURL(domain, orgExternalID string) string {
	return fmt.Sprintf("%s/%s", domain, orgExternalID)
}

func billingURL(domain, orgExternalID string) string {
	return fmt.Sprintf("%s/%s/org/billing", domain, orgExternalID)
}

func collectEmails(users []*users.User) []string {
	var e []string
	for _, u := range users {
		e = append(e, u.Email)
	}
	return e
}
