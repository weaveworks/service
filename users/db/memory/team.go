package memory

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/weaveworks/service/users"

	"github.com/weaveworks/service/users/externalids"
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
		externalID = externalids.Generate()
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
	if name == "" {
		return nil, errors.New("Team name cannot be blank")
	}

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

// ensureUserIsPartOfTeamByExternalID ensures the users is part of an existing team
func (d DB) ensureUserIsPartOfTeamByExternalID(ctx context.Context, userID, teamExternalID string) (*users.Team, error) {
	// no lock needed: called by CreateOrganization which acquired the lock
	if teamExternalID == "" {
		return nil, errors.New("teamExternalID must be provided")
	}

	var team *users.Team
	for _, t := range d.teams {
		if t.ExternalID == teamExternalID {
			team = t
			break
		}
	}

	if team == nil {
		return nil, fmt.Errorf("team does not exist: %v", teamExternalID)
	}

	for _, teamID := range d.teamMemberships[userID] {
		if teamID == team.ID {
			// user is part of team
			return d.teams[teamID], nil
		}
	}

	err := d.AddUserToTeam(ctx, userID, team.ID)
	if err != nil {
		return nil, err
	}

	return team, nil
}

// ensureUserIsPartOfTeamByName ensures the users is part of team by name, the team is created if it does not exist
func (d DB) ensureUserIsPartOfTeamByName(ctx context.Context, userID, teamName string) (*users.Team, error) {
	if teamName == "" {
		return nil, errors.New("teamName must be provided")
	}

	for _, teamID := range d.teamMemberships[userID] {
		team := d.teams[teamID]
		if strings.ToLower(teamName) == strings.ToLower(team.Name) {
			// user is part of the team
			return team, nil
		}
	}

	// teams does not exists for the user, create it!
	team, err := d.CreateTeam(ctx, teamName)
	if err != nil {
		return nil, err
	}

	err = d.AddUserToTeam(ctx, userID, team.ID)
	if err != nil {
		return nil, err
	}

	return team, nil
}
