package emailer_test

import (
	"testing"
	"time"

	"github.com/jordan-wright/email"
	"github.com/stretchr/testify/assert"

	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/emailer"
	"github.com/weaveworks/service/users/templates"
)

func createEmailer(sender func(*email.Email) error) emailer.SMTPEmailer {
	templates := templates.MustNewEngine("../templates")
	return emailer.SMTPEmailer{
		Templates:   templates,
		Domain:      "https://weave.test",
		FromAddress: "from@weave.test",
		Sender:      sender,
	}
}

func TestTrialPendingExpiryEmail(t *testing.T) {
	var sent bool
	expires := time.Now().Add(10 * 24 * time.Hour)
	em := createEmailer(func(e *email.Email) error {
		assert.Equal(t, "Your Weave Cloud trial expires soon!", e.Subject)
		assert.Equal(t, "from@weave.test", e.From)
		assert.Len(t, e.To, 2)
		assert.Contains(t, e.To, "user1@weave.test")
		assert.Contains(t, e.To, "user2@weave.test")
		assert.Contains(t, string(e.Text), "Test Org")
		assert.Contains(t, string(e.Text), expires.Format("January 2 2006"))
		assert.Contains(t, string(e.Text), "in 10 days")
		assert.Contains(t, string(e.Text), "https://weave.test/foo-boo-12/org/billing")
		sent = true
		return nil
	})

	receivers := []*users.User{{Email: "user1@weave.test"}, {Email: "user2@weave.test"}}
	err := em.TrialPendingExpiryEmail(receivers, "foo-boo-12", "Test Org", expires)
	assert.NoError(t, err)
	assert.True(t, sent, "email has not been sent")
}

func TestSMTPEmailer_TrialExpiredEmail(t *testing.T) {
	var sent bool
	em := createEmailer(func(e *email.Email) error {
		assert.Equal(t, "Your Weave Cloud trial expired", e.Subject)
		assert.Equal(t, "from@weave.test", e.From)
		assert.Len(t, e.To, 1)
		assert.Contains(t, e.To, "user@weave.test")
		assert.Contains(t, string(e.Text), "Test Org")
		assert.Contains(t, string(e.Text), "https://weave.test/foo-boo-12/org/billing")
		sent = true
		return nil
	})

	receivers := []*users.User{{Email: "user@weave.test"}}
	err := em.TrialExpiredEmail(receivers, "foo-boo-12", "Test Org")
	assert.NoError(t, err)
	assert.True(t, sent, "email has not been sent")
}

func TestSMTPEmailer_TrialExtendedEmail(t *testing.T) {
	var sent bool
	expires := time.Now().Add(15 * 24 * time.Hour)
	em := createEmailer(func(e *email.Email) error {
		assert.Equal(t, "15 days left of your free trial", e.Subject)
		assert.Equal(t, "from@weave.test", e.From)
		assert.Len(t, e.To, 1)
		assert.Contains(t, e.To, "user@weave.test")
		text := string(e.Text)
		assert.Contains(t, text, "Test Org")
		assert.Contains(t, text, "15 days")
		assert.Contains(t, text, expires.Format("January 2 2006"))
		assert.Contains(t, text, "https://weave.test/foo-boo-12/org/billing")
		sent = true
		return nil
	})

	receivers := []*users.User{{Email: "user@weave.test"}}
	err := em.TrialExtendedEmail(receivers, "foo-boo-12", "Test Org", expires)
	assert.NoError(t, err)
	assert.True(t, sent, "email has not been sent")
}
