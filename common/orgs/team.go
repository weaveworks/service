package orgs

import (
	"fmt"
	"strings"
)

const (
	defaultTeamNameTemplate = "%v Team"
)

// TeamNameFromOrgExternalID returns a team name derived from an org's externalID
func TeamNameFromOrgExternalID(externalID string) string {
	return fmt.Sprintf(defaultTeamNameTemplate, strings.Title(strings.Replace(externalID, "-", " ", -1)))
}
