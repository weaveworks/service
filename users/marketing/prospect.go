package marketing

import "time"

// Prospect gathers meta-data about users who signed up into Weave Cloud.
type Prospect struct {
	Email             string    `json:"email"`
	SignupSource      string    `json:"signupSource"`
	ServiceCreatedAt  time.Time `json:"createdAt"`
	ServiceLastAccess time.Time `json:"lastAccess"`
	CampaignID        string    `json:"campaignId"`
	LeadSource        string    `json:"leadSource"`

	// TODO: these don't fit well in this prospect system. We should redesign the whole thing.
	OrganizationBillingConfiguredExternalID string `json:"organizationBillingConfiguredExternalID"`
	OrganizationBillingConfiguredName       string `json:"organizationBillingConfiguredName"`
}

// Merge merges this prospect with the provided one, creating a new prospect.
func (p1 Prospect) Merge(p2 Prospect) Prospect {
	return Prospect{
		Email:             either(p1.Email, p2.Email),
		SignupSource:      either(p1.SignupSource, p2.SignupSource),
		ServiceCreatedAt:  latest(p1.ServiceCreatedAt, p2.ServiceCreatedAt),
		ServiceLastAccess: latest(p1.ServiceLastAccess, p2.ServiceLastAccess),
		CampaignID:        either(p1.CampaignID, p2.CampaignID),
		LeadSource:        either(p1.LeadSource, p2.LeadSource),

		OrganizationBillingConfiguredExternalID: either(p1.OrganizationBillingConfiguredExternalID, p2.OrganizationBillingConfiguredExternalID),
		OrganizationBillingConfiguredName:       either(p1.OrganizationBillingConfiguredName, p2.OrganizationBillingConfiguredName),
	}
}

func either(s1, s2 string) string {
	if s1 != "" {
		return s1
	}
	return s2
}

func latest(t1, t2 time.Time) time.Time {
	if t1.After(t2) {
		return t1
	}
	return t2
}
