package users

import (
	"github.com/weaveworks/service/common/featureflag"
)

// NewSummaryEntry creates a new summary entry by merging the provided organization, team and users.
func NewSummaryEntry(org *Organization, team *Team, users []*User) *SummaryEntry {
	entry := SummaryEntry{
		TeamExternalID:               team.ExternalID,
		TeamName:                     team.Name,
		OrgID:                        org.ID,
		OrgExternalID:                org.ExternalID,
		OrgName:                      org.Name,
		Emails:                       emails(users),
		OrgCreatedAt:                 org.CreatedAt,
		FirstSeenConnectedAt:         org.FirstSeenConnectedAt,
		Platform:                     org.Platform,
		Environment:                  org.Environment,
		TrialExpiresAt:               org.TrialExpiresAt,
		TrialPendingExpiryNotifiedAt: org.TrialPendingExpiryNotifiedAt,
		TrialExpiredNotifiedAt:       org.TrialExpiredNotifiedAt,
		BillingEnabled:               org.HasFeatureFlag(featureflag.Billing),
		RefuseDataAccess:             org.RefuseDataAccess,
		RefuseDataUpload:             org.RefuseDataUpload,
		ZuoraAccountNumber:           org.ZuoraAccountNumber,
		ZuoraAccountCreatedAt:        org.ZuoraAccountCreatedAt,
	}
	if org.GCP != nil {
		entry.GCPAccountExternalID = org.GCP.ExternalAccountID
		entry.GCPAccountCreatedAt = org.GCP.CreatedAt
		entry.GCPAccountSubscriptionLevel = org.GCP.SubscriptionLevel
		entry.GCPAccountSubscriptionStatus = org.GCP.SubscriptionStatus
	}
	return &entry
}

func emails(users []*User) []string {
	emails := []string{}
	for _, user := range users {
		emails = append(emails, user.Email)
	}
	return emails
}

// SummaryEntriesByCreatedAt helps sort an array of summary entries by organization's creation date.
type SummaryEntriesByCreatedAt []*SummaryEntry

func (s SummaryEntriesByCreatedAt) Len() int      { return len(s) }
func (s SummaryEntriesByCreatedAt) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s SummaryEntriesByCreatedAt) Less(i, j int) bool {
	return s[i].OrgCreatedAt.After(s[j].OrgCreatedAt)
}
