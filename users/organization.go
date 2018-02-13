package users

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/weaveworks/service/users/tokens"
)

const (
	// TrialDuration is how long an organization has a free trial
	// period before we start charging for it.
	TrialDuration = 30 * 24 * time.Hour

	// TrialExtensionDuration is the extension period if billing is
	// enabled for an existing customer
	TrialExtensionDuration = 15 * 24 * time.Hour

	// BillingFeatureFlag enables billing for an organization
	BillingFeatureFlag = "billing"

	defaultTeamNameTemplate = "%v Team"
)

var (
	orgExternalIDRegex = regexp.MustCompile(`\A[a-zA-Z0-9_-]+\z`)
	// Must be kept in sync with service-ui/client/src/content/environments.json
	platforms = map[string]map[string]struct{}{
		"kubernetes": {
			"minikube": struct{}{},
			"gke":      struct{}{},
			"generic":  struct{}{},
		},
		"docker": {
			"mac":     struct{}{},
			"linux":   struct{}{},
			"windows": struct{}{},
			"ee":      struct{}{},
			"swarm":   struct{}{},
		},
		"ecs": {
			"aws": struct{}{},
		},
		"dcos": {
			"mesosphere": struct{}{},
		},
	}
)

// Membership represents a users membership of an organization.
type Membership struct {
	UserID         string
	OrganizationID string
}

// OrgWriteView represents an update for an organization with optional fields.
// A nil field is not updating the value for the organization.
type OrgWriteView struct {
	Name           *string
	Platform       *string
	Environment    *string
	TrialExpiresAt *time.Time

	// These time values are nullable in the database but cannot be set to NULL
	// through this struct.
	TrialPendingExpiryNotifiedAt *time.Time
	TrialExpiredNotifiedAt       *time.Time
}

// RegenerateProbeToken regenerates the organizations probe token
func (o *Organization) RegenerateProbeToken() error {
	t, err := tokens.Generate()
	if err != nil {
		return err
	}
	o.ProbeToken = t
	return nil
}

// InTrialPeriod determines whether this organization is within its trial period.
func (o *Organization) InTrialPeriod(now time.Time) bool {
	return o.TrialExpiresAt.After(now)
}

// Valid check if the organization is valid. Good to call before saving.
func (o *Organization) Valid() error {
	switch {
	case o.ExternalID == "":
		return ErrOrgExternalIDCannotBeBlank
	case !orgExternalIDRegex.MatchString(o.ExternalID):
		return ErrOrgExternalIDFormat
	case o.Name == "":
		return ErrOrgNameCannotBeBlank
	}

	// Check platform and environment
	if o.Platform == "" && o.Environment != "" {
		return ErrOrgPlatformRequired
	}
	if o.Environment == "" && o.Platform != "" {
		return ErrOrgEnvironmentRequired
	}

	environments, ok := platforms[o.Platform]
	if o.Platform != "" && !ok {
		return ErrOrgPlatformInvalid
	}
	_, ok = environments[o.Environment]
	if o.Environment != "" && !ok {
		return ErrOrgEnvironmentInvalid
	}

	if o.TrialExpiresAt.IsZero() {
		return ErrOrgTrialExpiresInvalid
	}

	return nil
}

// FormatCreatedAt formats the org's created at timestamp
func (o *Organization) FormatCreatedAt() string {
	return formatTimestamp(o.CreatedAt)
}

// HasFeatureFlag returns true if the organization has the given feature flag.
func (o *Organization) HasFeatureFlag(needle string) bool {
	for _, f := range o.FeatureFlags {
		if f == needle {
			return true
		}
	}
	return false
}

// BillingProvider returns the name of the provider processing billing.
// It is `zuora` by default.
func (o *Organization) BillingProvider() string {
	if o.GCP != nil {
		return "gcp"
	}
	return "zuora"
}

// IsOnboarded returns whether the organization has onboarded
func (o *Organization) IsOnboarded() bool {
	return o.FirstSeenConnectedAt != nil
}

// DefaultOrganizationName returns the default name which is derived from
// the externalID.
func DefaultOrganizationName(externalID string) string {
	return strings.Title(strings.Replace(externalID, "-", " ", -1))
}

// DefaultTeamName returns the default name which is derived from
// the organization externalID.
func DefaultTeamName(orgExternalID string) string {
	return fmt.Sprintf(defaultTeamNameTemplate, DefaultOrganizationName(orgExternalID))
}
