package users

import (
	"regexp"
	"time"

	"github.com/weaveworks/service/users/tokens"
)

var (
	orgExternalIDRegex = regexp.MustCompile(`\A[a-zA-Z0-9_-]+\z`)
)

// Organization (aka Instance) represents a database organization, and a UI
// instance. (same thing)
type Organization struct {
	ID                 string
	ExternalID         string
	Name               string
	ProbeToken         string
	FirstProbeUpdateAt time.Time
	CreatedAt          time.Time
	FeatureFlags       []string
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
	return nil
}
