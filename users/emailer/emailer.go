package emailer

import (
	"errors"
	"fmt"
	"net/url"

	log "github.com/Sirupsen/logrus"
	"github.com/jordan-wright/email"

	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/templates"
)

// ErrUnsupportedEmailProtocol is the error when an email protocol is unsupported.
var ErrUnsupportedEmailProtocol = errors.New("Unsupported email protocol")

func loginURL(email, rawToken, domain string) string {
	return fmt.Sprintf(
		"%s/login/%s/%s",
		domain,
		url.QueryEscape(email),
		url.QueryEscape(rawToken),
	)
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
	return fmt.Sprintf("%s/org/%s", domain, orgExternalID)
}

// Emailer is the interface which emailers implement. There should be a method
// for each type of email we send.
type Emailer interface {
	LoginEmail(u *users.User, token string) error
	InviteEmail(inviter, invited *users.User, orgExternalID, orgName, token string) error
	GrantAccessEmail(inviter, invited *users.User, orgExternalID, orgName string) error
}

// MustNew creates a new emailer, from the URI, or panics.
func MustNew(emailURI, fromAddress string, templates templates.Engine, domain string) Emailer {
	if emailURI == "" {
		log.Fatal("Must -email-uri")
	}
	var sender func(*email.Email) error
	u, err := url.Parse(emailURI)
	if err != nil {
		log.Fatal(fmt.Errorf("Error parsing -email-uri: %s", err))
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
