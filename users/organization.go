package users

import (
	"regexp"

	"github.com/weaveworks/service/users/tokens"
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
type OrgWriteView struct {
	Name        *string
	Platform    *string
	Environment *string
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
