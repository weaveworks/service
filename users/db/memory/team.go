package memory

import (
	"context"

	"github.com/weaveworks/service/users"
)

// ListTeamsForUserID lists the teams these users belong to
func (d *DB) ListTeamsForUserID(_ context.Context, userID string) ([]*users.Team, error) {
	d.mtx.Lock()
	defer d.mtx.Unlock()

	var teams []*users.Team
	teamIDs, exists := d.teamMemberships[userID]
	if !exists {
		return teams, nil
	}

	for _, teamID := range teamIDs {
		team := d.teams[teamID]
		teams = append(teams, team)
	}

	return teams, nil
}
