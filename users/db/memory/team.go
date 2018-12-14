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

// ListRoles lists all user roles
func (d *DB) ListRoles(_ context.Context) ([]*users.Role, error) {
	d.mtx.Lock()
	defer d.mtx.Unlock()

	var roles []*users.Role
	for _, role := range d.roles {
		roles = append(roles, role)
	}

	return roles, nil
}

// ListTeamsForUserID lists the teams these users belong to
func (d *DB) ListTeamsForUserID(_ context.Context, userID string) ([]*users.Team, error) {
	d.mtx.Lock()
	defer d.mtx.Unlock()

	var teams []*users.Team
	for teamID := range d.teamMemberships[userID] {
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
	for m, teamRoles := range d.teamMemberships {
		for tID := range teamRoles {
			if tID == teamID {
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
	// no lock needed: caller must acquire lock.
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
	// no lock needed: caller must acquire lock.
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
	// no lock needed: caller must acquire lock.
	teamRoles, _ := d.teamMemberships[userID]
	for tID := range teamRoles {
		if tID == teamID {
			return nil
		}
	}
	// Make sure the submap has been initialized.
	if teamRoles == nil {
		teamRoles = map[string]string{}
	}
	// TODO(fbarl): Change this to 'viewer' once permissions UI is in place.
	teamRoles[teamID] = "admin"
	d.teamMemberships[userID] = teamRoles
	return nil
}

// DeleteTeam marks the given team as deleted.
func (d *DB) DeleteTeam(ctx context.Context, teamID string) error {
	// Verify team has no orgs
	for _, org := range d.organizations {
		if org.TeamID == teamID {
			return users.ErrForbidden
		}
	}

	// Delete memberships
	for uid, teamRoles := range d.teamMemberships {
		for tid := range teamRoles {
			if tid == teamID {
				delete(d.teamMemberships[uid], tid)
			}
		}
	}

	// Delete team
	delete(d.teams, teamID)

	return nil
}

// GetUserRoleInTeam returns the role the given user has in the given team
func (d *DB) GetUserRoleInTeam(_ context.Context, userID, teamID string) (*users.Role, error) {
	roleID, exists := d.teamMemberships[userID][teamID]
	if !exists {
		return nil, fmt.Errorf("user %v is not part of the team %v", userID, teamID)
	}
	return d.roles[roleID], nil
}

// UpdateUserRoleInTeam updates the role the given user has in the given team
func (d *DB) UpdateUserRoleInTeam(_ context.Context, userID, teamID, roleID string) error {
	if _, exists := d.teamMemberships[userID][teamID]; !exists {
		return fmt.Errorf("user %v is not part of the team %v", userID, teamID)
	}
	d.teamMemberships[userID][teamID] = roleID
	return nil
}

// getTeamUserIsPartOf returns the team the user is part of.
func (d *DB) getTeamUserIsPartOf(ctx context.Context, userID, teamExternalID string) (*users.Team, error) {
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

	for teamID := range d.teamMemberships[userID] {
		if teamID == team.ID {
			// user is part of team
			return d.teams[teamID], nil
		}
	}

	return nil, users.ErrNotFound
}

// ensureUserIsPartOfTeamByName ensures the users is part of team by name, the team is created if it does not exist
func (d *DB) ensureUserIsPartOfTeamByName(ctx context.Context, userID, teamName string) (*users.Team, error) {
	// no lock needed: caller must acquire lock.
	if teamName == "" {
		return nil, errors.New("teamName must be provided")
	}

	for teamID := range d.teamMemberships[userID] {
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
