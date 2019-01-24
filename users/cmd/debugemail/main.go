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
	"github.com/weaveworks/service/users/weeklyreports"
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
	weeklyReport := &weeklyreports.Report{
		GeneratedAt: time.Now(),
		StartAt:     time.Now().UTC().Truncate(24*time.Hour).AddDate(0, 0, -7),
		EndAt:       time.Now().UTC().Truncate(24 * time.Hour),
		Organization: &users.Organization{
			Name:       "Sample Org 99",
			ExternalID: "sample-org-99",
			CreatedAt:  time.Now().AddDate(0, 0, -6), // Simulate instance creation after first day of the report.
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
	orgExternalID := "sample-org-99"
	orgName := "Sample Org 99"
	teamExternalID := "sample-team-88"
	teamName := "Sample Team 88"

	actions := map[string]func() error{
		"login": func() error {
			return em.LoginEmail(dst, "secret-login-token", nil)
		},
		"invite": func() error {
			return em.InviteToTeamEmail(inviter, dst, teamExternalID, teamName, "secret-invite-token")
		},
		"grant_access": func() error {
			return em.GrantAccessToTeamEmail(inviter, dst, teamExternalID, teamName)
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
			return em.WeeklyReportEmail([]*users.User{dst}, weeklyReport)
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
