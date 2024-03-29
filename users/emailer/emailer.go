package emailer

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/jordan-wright/email"
	log "github.com/sirupsen/logrus"

	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/templates"
	"github.com/weaveworks/service/users/weeklyreports"
)

// ErrUnsupportedEmailProtocol is the error when an email protocol is unsupported.
var ErrUnsupportedEmailProtocol = errors.New("unsupported email protocol")

// Emailer is the interface which emailers implement. There should be a method
// for each type of email we send.
type Emailer interface {
	InviteToTeamEmail(ctx context.Context, inviter, invited *users.User, teamExternalID, teamName, token string) error
	GrantAccessToTeamEmail(ctx context.Context, inviter, invited *users.User, teamExternalID, teamName string) error
	TrialExtendedEmail(ctx context.Context, members []*users.User, orgExternalID, orgName string, expiresAt time.Time) error
	TrialPendingExpiryEmail(ctx context.Context, members []*users.User, orgExternalID, orgName string, expiresAt time.Time) error
	TrialExpiredEmail(ctx context.Context, members []*users.User, orgExternalID, orgName string) error
	RefuseDataUploadEmail(ctx context.Context, members []*users.User, orgExternalID, orgName string) error
	WeeklyReportEmail(ctx context.Context, members []*users.User, report *weeklyreports.Report) error
}

// MustNew creates a new Emailer, from the URI, or panics.
// Supports scheme smtp:// and log:// in `emailURI`.
func MustNew(emailURI, fromAddress string, templates templates.Engine, domain string) Emailer {
	var sendDirectly func(context.Context, *email.Email) error
	u, err := url.Parse(emailURI)
	if err != nil {
		log.Fatalf("Error parsing -email-uri: %s", err)
	}
	switch u.Scheme {
	case "smtp":
		sendDirectly, err = smtpEmailSender(u)
	case "log":
		sendDirectly = logEmailSender()
	default:
		err = ErrUnsupportedEmailProtocol
	}
	if err != nil {
		log.Fatal(err)
	}
	return newSMTPEmailer(fromAddress, templates, domain, sendDirectly)
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

func inviteToTeamURL(email, rawToken, domain, teamName string) string {
	return fmt.Sprintf(
		"%s/login/%s/%s/%s",
		domain,
		teamName,
		url.QueryEscape(email),
		url.QueryEscape(rawToken),
	)
}

func organizationURL(domain, orgExternalID string) string {
	return fmt.Sprintf("%s/%s", domain, orgExternalID)
}

func teamURL(domain, teamExternalID string) string {
	return fmt.Sprintf("%s/instances#%s", domain, teamExternalID)
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
