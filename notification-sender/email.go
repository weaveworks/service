package sender

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/weaveworks/blackfriday"
	"github.com/weaveworks/service/notification-eventmanager/types"
	gomail "gopkg.in/gomail.v2"
)

const (
	// Timeout waiting for mail service to be ready
	timeout              = 5 * time.Minute
	markdownNewline      = "  \n"
	markdownNewParagraph = "\n\n"

	// EmailSeparator divides addresses in receiver configuration.
	EmailSeparator = ","
)

// EmailSender contains creds to send emails
type EmailSender struct {
	URI     string
	From    string
	ReplyTo string
}

func waitForMailService(uri, from, replyto string) error {
	deadline := time.Now().Add(timeout)
	var err error
	for tries := 0; time.Now().Before(deadline); tries++ {
		err = parseAndSend(uri, from, replyto, []string{"weaveworkstest@gmail.com"}, "Email sender validation", from)
		if err == nil {
			return nil
		}
		log.Debugf("mail service not ready, error: %s; retrying...", err)
		time.Sleep(time.Second << uint(tries))
	}
	return errors.Errorf("mail service not ready after %s, error: %s", timeout, err)
}

// ValidateEmailSender validates uri and from for email sender by sending test email
func ValidateEmailSender(uri, from, replyto string) error {
	if err := waitForMailService(uri, from, replyto); err != nil {
		return errors.Wrap(err, "email sender validation failed")
	}
	log.Debug("email sender validated successfully")
	return nil
}

// Send sends data to address with EmailSender creds
func (es *EmailSender) Send(_ context.Context, addr json.RawMessage, notif types.Notification, _ string) error {
	var addrStr string
	if err := json.Unmarshal(addr, &addrStr); err != nil {
		return errors.Wrapf(err, "cannot unmarshal address %s", addr)
	}

	var notifData types.EmailMessage
	var err error
	// See if we should use the new Event schema.
	// Handle the formatting for the client (event creator)
	// https://github.com/weaveworks/service/issues/1791
	if useNewNotifSchema(notif) {
		// Using new Event schema
		notifData, err = generateEmailMessage(notif.Event)
	} else {
		err = json.Unmarshal(notif.Data, &notifData)
	}

	if err != nil {
		return err
	}

	if es.URI == "" {
		return errors.New("cannot create email sender, email URI is empty")
	}

	addresses := strings.Split(addrStr, EmailSeparator)
	if err := parseAndSend(es.URI, es.From, es.ReplyTo, addresses, notifData.Subject, notifData.Body); err != nil {
		return errors.Wrap(err, "cannot parse and send email")
	}

	return nil
}

func parseAndSend(uri, from string, replyto string, addresses []string, subject, body string) error {
	u, err := url.Parse(uri)
	if err != nil {
		return errors.Wrapf(err, "cannot parse email URI %s", uri)
	}

	switch u.Scheme {
	case "smtp":
		strPort := u.Port()
		var port int
		if strPort == "" {
			port = 587
			log.Info("SMTP port is empty, use port 587 by default")
		} else {
			port, err = strconv.Atoi(strPort)
			if err != nil {
				return errors.Errorf("cannot convert port %s to integer", strPort)
			}
		}

		var username, password string
		if u.User != nil {
			username = u.User.Username()
			password, _ = u.User.Password()
		}

		d := gomail.NewPlainDialer(u.Hostname(), port, username, password)
		m := gomail.NewMessage()
		var formatted []string
		for _, a := range addresses {
			formatted = append(formatted, m.FormatAddress(a, ""))
		}
		m.SetHeader("From", from)
		m.SetHeader("To", formatted...)
		m.SetHeader("Subject", subject)
		m.SetHeader("Reply-To", replyto)
		m.SetBody("text/html", body)
		uid := uuid.New()
		m.SetHeader("X-Entity-Ref-ID", uid.String())
		log.Debugf("[Email] From: %s, To: %s, Subject: %s, Body: %s", from, addresses, subject, body)

		if err := d.DialAndSend(m); err != nil {
			return errors.Wrap(err, "cannot create new SMTP dialer and send message")
		}

	case "log":
		log.Infof("[Email] From: %s, To: %s, Subject: %s, Body: %s", from, addresses, subject, body)

	default:
		return errors.Errorf("Unsupported email protocol: %s", u.Scheme)
	}

	return nil
}

func generateEmailMessage(e types.Event) (types.EmailMessage, error) {
	var msg bytes.Buffer
	text := *e.Text
	msg.WriteString(fmt.Sprintf("Instance: %s%s", e.InstanceName, markdownNewParagraph))
	if text != "" {
		// a slack message might contain \n for new lines
		// replace it with markdown line break
		msg.WriteString(strings.Replace(text, "\n", markdownNewline, -1))
		msg.WriteString(markdownNewParagraph)
	}

	html := string(blackfriday.MarkdownBasic([]byte(msg.String())))

	email := types.EmailMessage{
		Subject: fmt.Sprintf("%v - %v", e.InstanceName, e.Type),
		Body:    html,
	}

	return email, nil
}
