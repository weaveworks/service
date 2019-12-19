package emailer_test

import (
	"context"
	"testing"
	"time"

	"github.com/jordan-wright/email"
	"github.com/stretchr/testify/assert"

	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/emailer"
	"github.com/weaveworks/service/users/templates"
	"github.com/weaveworks/service/users/weeklyreports"
)

func createEmailer(sendDirectly func(context.Context, *email.Email) error) emailer.SMTPEmailer {
	templates := templates.MustNewEngine("../templates")
	return emailer.SMTPEmailer{
		Templates:    templates,
		Domain:       "https://weave.test",
		FromAddress:  "from@weave.test",
		SendDirectly: sendDirectly,
	}
}

func TestTrialPendingExpiryEmail(t *testing.T) {
	var sent bool
	expires := time.Now().Add(10 * 24 * time.Hour)
	em := createEmailer(func(ctx context.Context, e *email.Email) error {
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
	err := em.TrialPendingExpiryEmail(context.Background(), receivers, "foo-boo-12", "Test Org", expires)
	assert.NoError(t, err)
	assert.True(t, sent, "email has not been sent")
}

func TestSMTPEmailer_TrialExpiredEmail(t *testing.T) {
	var sent bool
	em := createEmailer(func(ctx context.Context, e *email.Email) error {
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
	err := em.TrialExpiredEmail(context.Background(), receivers, "foo-boo-12", "Test Org")
	assert.NoError(t, err)
	assert.True(t, sent, "email has not been sent")
}

func TestSMTPEmailer_TrialExtendedEmail(t *testing.T) {
	var sent bool
	expires := time.Now().Add(15 * 24 * time.Hour)
	em := createEmailer(func(ctx context.Context, e *email.Email) error {
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
	err := em.TrialExtendedEmail(context.Background(), receivers, "foo-boo-12", "Test Org", expires)
	assert.NoError(t, err)
	assert.True(t, sent, "email has not been sent")
}

func TestSMTPEmailer_WeeklyReportEmail(t *testing.T) {
	createdAt, _ := time.Parse(time.RFC3339, "2019-09-30T16:47:17Z")
	startAt, _ := time.Parse(time.RFC3339, "2019-09-30T00:00:00Z")
	endAt, _ := time.Parse(time.RFC3339, "2019-10-07T00:00:00Z")

	report := &weeklyreports.Report{
		GeneratedAt: time.Now(),
		StartAt:     startAt,
		EndAt:       endAt,
		Organization: &users.Organization{
			Name:       "Sample Org 99",
			ExternalID: "sample-org-99",
			CreatedAt:  createdAt,
		},
		DeploymentsPerDay: []int{12, 43, 18, 23, 4, 0, 1},
		CPUIntensiveWorkloads: []weeklyreports.WorkloadResourceConsumptionRaw{
			{
				WorkloadName:       "cortex:deployment/ingester",
				ClusterConsumption: 0.134,
			},
			{
				WorkloadName:       "cortex:deployment/distributor",
				ClusterConsumption: 0.0312,
			},
			{
				WorkloadName:       "monitoring:daemonset/fluentd-loggly",
				ClusterConsumption: 0.0103,
			},
		},
		MemoryIntensiveWorkloads: []weeklyreports.WorkloadResourceConsumptionRaw{
			{
				WorkloadName:       "monitoring:deployment/prometheus",
				ClusterConsumption: 0.2095,
			},
			{
				WorkloadName:       "cortex:deployment/ingester",
				ClusterConsumption: 0.1310,
			},
			{
				WorkloadName:       "monitoring:daemonset/fluentd-loggly",
				ClusterConsumption: 0.0345,
			},
		},
	}

	var sent bool
	em := createEmailer(func(ctx context.Context, e *email.Email) error {
		assert.Equal(t, "Sep 30 – Oct 6 · Sample Org 99 Report", e.Subject)
		assert.Equal(t, "from@weave.test", e.From)
		assert.Len(t, e.To, 1)
		assert.Contains(t, e.To, "user@weave.test")
		text := string(e.Text)
		assert.Contains(t, text, "You created `Sample Org 99` on September 30nd, 2019.")
		assert.Contains(t, text, "Mon: 12")
		assert.Contains(t, text, "Tue: 43")
		assert.Contains(t, text, "Wed: 18")
		assert.Contains(t, text, "Thu: 23")
		assert.Contains(t, text, "Fri: 4")
		assert.Contains(t, text, "Sat: 0")
		assert.Contains(t, text, "Sun: 1")
		assert.Contains(t, text, "cortex:deployment/ingester - 13.40%")
		assert.Contains(t, text, "cortex:deployment/distributor - 3.12%")
		assert.Contains(t, text, "monitoring:daemonset/fluentd-loggly - 1.03%")
		assert.Contains(t, text, "monitoring:deployment/prometheus - 20.95%")
		assert.Contains(t, text, "cortex:deployment/ingester - 13.10%")
		assert.Contains(t, text, "monitoring:daemonset/fluentd-loggly - 3.45%")
		sent = true
		return nil
	})

	receivers := []*users.User{{Email: "user@weave.test"}}
	err := em.WeeklyReportEmail(context.Background(), receivers, report)
	assert.NoError(t, err)
	assert.True(t, sent, "email has not been sent")
}
