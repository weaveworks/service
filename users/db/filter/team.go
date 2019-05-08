package filter

import (
	"fmt"
	"github.com/Masterminds/squirrel"
	"strings"

	"github.com/weaveworks/service/users"
)

// Team filters teams.
type Team interface {
	Filter
	// MatchesTeam checks whether a team matches this filter.
	MatchesTeam(users.Team) bool
}

// TeamID filters for teams with exactly this ID.
type TeamID string

// Where implements Team.
func (i TeamID) Where() squirrel.Sqlizer {
	return squirrel.Eq{"teams.id": string(i)}
}

// MatchesTeam implements Team.
func (i TeamID) MatchesTeam(t users.Team) bool {
	return t.ID == string(i)
}

// TeamExternalID filters for teams with exactly this external ID.
type TeamExternalID string

// Where implements Team.
func (e TeamExternalID) Where() squirrel.Sqlizer {
	return squirrel.Eq{"teams.external_id": string(e)}
}

// MatchesTeam implements Team.
func (e TeamExternalID) MatchesTeam(t users.Team) bool {
	return t.ExternalID == string(e)
}

// TeamName finds teams that have names that contain a string.
type TeamName string

// Where implements Team.
func (n TeamName) Where() squirrel.Sqlizer {
	return squirrel.Expr("lower(teams.name) LIKE ?",
		fmt.Sprint("%", strings.ToLower(string(n)), "%"))
}

// MatchesTeam implements Team.
func (n TeamName) MatchesTeam(t users.Team) bool {
	return strings.Contains(t.Name, string(n))
}
