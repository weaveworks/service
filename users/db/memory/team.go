package memory

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/weaveworks/service/users"

	"github.com/weaveworks/service/users/externalIDs"
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

// ListTeamUsers lists all the users in an team
func (d *DB) ListTeamUsers(ctx context.Context, teamID string) ([]*users.User, error) {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	return d.listTeamUsers(ctx, teamID)
}

func (d *DB) listTeamUsers(_ context.Context, teamID string) ([]*users.User, error) {
	var users []*users.User
	for m, teamIDs := range d.teamMemberships {
		for _, teamID := range teamIDs {
			if teamID == teamID {
				u, err := d.findUserByID(m)
				if err != nil {
					return nil, err
				}
				users = append(users, u)
			}
		}
	}

	sort.Sort(usersByCreatedAt(users))
	return users, nil
}

func (d *DB) generateTeamExternalID(_ context.Context) (string, error) {
	// no lock needed: called by createTeam which acquired the lock
	var externalID string
	for used := true; used; {
		externalID = externalIDs.Generate()
		if len(d.teams) == 0 {
			break
		}
		for _, team := range d.teams {
			if team.ExternalID != externalID {
				used = false
			}
		}
	}
	return externalID, nil
}

// CreateTeam creates a team
func (d *DB) CreateTeam(ctx context.Context, name string) (*users.Team, error) {
	// no lock needed: called by CreateOrganization which acquired the lock
	now := time.Now()
	TrialExpiresAt := now.Add(users.TrialDuration)
	t := &users.Team{
		ID:             fmt.Sprint(len(d.teams)),
		Name:           name,
		TrialExpiresAt: TrialExpiresAt,
	}
	externalID, err := d.generateTeamExternalID(ctx)
	if err != nil {
		return nil, err
	}
	t.ExternalID = externalID

	d.teams[t.ID] = t
	return t, nil
}

// AddUserToTeam links a user to the team
func (d *DB) AddUserToTeam(_ context.Context, userID, teamID string) error {
	// no lock needed: called by CreateOrganization which acquired the lock
	teamIDs, _ := d.teamMemberships[userID]
	for _, id := range teamIDs {
		if id == teamID {
			return nil
		}
	}
	teamIDs = append(teamIDs, teamID)
	d.teamMemberships[userID] = teamIDs
	return nil
}

func (d *DB) setDefaultTeam(userID, teamID string) error {
	// no lock needed: called by CreateOrganization which acquired the lock
	teamIDs, _ := d.teamMemberships[userID]
	idx := -1
	for i, id := range teamIDs {
		idx = i
		if id == teamID {
			break
		}
	}
	if idx == -1 {
		return fmt.Errorf("Team %v not found, available teams: %v", teamID, teamIDs)
	}
	// make the first element the default
	teamIDs = append([]string{teamID}, append(teamIDs[:idx], teamIDs[idx+1:]...)...)
	d.teamMemberships[userID] = teamIDs
	return nil
}
