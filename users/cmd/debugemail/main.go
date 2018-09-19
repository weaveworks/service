// debugemail is a small tool to test email template rendering.
//
// Usage:
//
//   cd users
//   go run cmd/debugemail/main.go -email=<destination-email> -email-uri=<smtp-uri> <action>
//
// Example:
//
//   go run cmd/debugemail/main.go -email=foo@weave.works invite
//
package main

import (
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/emailer"
	"github.com/weaveworks/service/users/templates"
	"github.com/weaveworks/service/users/weekly-summary"
)

func main() {
	var (
		emailURI = flag.String("email-uri", "log://", "URI of smtp server to send email through, of the format: smtp://username:password@hostname:port.")
		email    = flag.String("email", "", "email TO address")
	)
	flag.Parse()
	if *email == "" {
		panic("missing -email")
	}

	tpls := templates.MustNewEngine("templates")
	em := emailer.MustNew(*emailURI, "support@weave.works", tpls, "https://weave.works.example")

	dst := &users.User{
		ID:    "123",
		Email: *email,
		Token: "user-token",
	}
	inviter := &users.User{
		ID:    "456",
		Email: "inviter@weave.works.example",
	}
	weeklyReport := &weeklySummary.Report{
		WorkloadReleasesCount: 200,
		CPUIntensiveWorkloads: []weeklySummary.WorkloadResourceConsumption{
			weeklySummary.WorkloadResourceConsumption{
				Name:  "cortex:deployment/ingester",
				Value: "10.34%",
			},
			weeklySummary.WorkloadResourceConsumption{
				Name:  "cortex:deployment/distributor",
				Value: "3.12%",
			},
			weeklySummary.WorkloadResourceConsumption{
				Name:  "monitoring:daemonset/fluentd-loggly",
				Value: "1.03%",
			},
		},
		MemoryIntensiveWorkloads: []weeklySummary.WorkloadResourceConsumption{
			weeklySummary.WorkloadResourceConsumption{
				Name:  "monitoring:deployment/prometheus",
				Value: "20.95%",
			},
			weeklySummary.WorkloadResourceConsumption{
				Name:  "cortex:deployment/ingester",
				Value: "13.10%",
			},
			weeklySummary.WorkloadResourceConsumption{
				Name:  "monitoring:daemonset/fluentd-loggly",
				Value: "3.45%",
			},
		},
	}
	orgExternalID := "sample-org-99"
	orgName := "Sample Org 99"

	actions := map[string]func() error{
		"login": func() error {
			return em.LoginEmail(dst, "secret-login-token", nil)
		},
		"invite": func() error {
			return em.InviteEmail(inviter, dst, orgExternalID, orgName, "secret-invite-token")
		},
		"grant_access": func() error {
			return em.GrantAccessEmail(inviter, dst, orgExternalID, orgName)
		},
		"trial_extended": func() error {
			return em.TrialExtendedEmail([]*users.User{dst}, orgExternalID, orgName, time.Now().Add(15*24*time.Hour))
		},
		"trial_pending_expiry": func() error {
			return em.TrialPendingExpiryEmail([]*users.User{dst}, orgExternalID, orgName, time.Now().Add(3*24*time.Hour))
		},
		"trial_expired": func() error {
			return em.TrialExpiredEmail([]*users.User{dst}, orgExternalID, orgName)
		},
		"weekly": func() error {
			return em.WeeklySummaryEmail(dst, orgExternalID, orgName, weeklyReport)
		},
	}

	action := actions[flag.Arg(0)]
	if action == nil {
		var names []string
		for a := range actions {
			names = append(names, a)
		}
		panic("error: unknown action, use one of " + strings.Join(names, ", "))
	}

	if err := action(); err != nil {
		panic(err)
	}

	fmt.Printf("successsfully sent '%s' email to %s\n", flag.Arg(0), *email)
}
