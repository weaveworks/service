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

type teamsByCreatedAt []*users.Team

func (o teamsByCreatedAt) Len() int           { return len(o) }
func (o teamsByCreatedAt) Swap(i, j int)      { o[i], o[j] = o[j], o[i] }
func (o teamsByCreatedAt) Less(i, j int) bool { return o[i].CreatedAt.After(o[j].CreatedAt) }

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

// ListTeams lists teams. It ignores pagination.
func (d *DB) ListTeams(_ context.Context, page uint64) ([]*users.Team, error) {
	d.mtx.Lock()
	defer d.mtx.Unlock()

	var teams []*users.Team
	for _, team := range d.teams {
		teams = append(teams, team)
	}
	sort.Sort(teamsByCreatedAt(teams))
	return teams, nil
}

// ListTeamUsersWithRoles lists all the users in a team with their role
func (d *DB) ListTeamUsersWithRoles(ctx context.Context, teamID string) ([]*users.UserWithRole, error) {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	return d.listTeamUsersWithRoles(ctx, teamID)
}

func (d *DB) listTeamUsersWithRoles(ctx context.Context, teamID string) ([]*users.UserWithRole, error) {
	var us []*users.User
	roles := map[string]*users.Role{}
	for m, teamRoles := range d.teamMemberships {
		for tID, rID := range teamRoles {
			if tID == teamID {
				u, err := d.findUserByID(m)
				if err != nil {
					return nil, err
				}
				us = append(us, u)
				r := d.roles[rID]
				roles[m] = r
			}
		}
	}

	sort.Sort(usersByCreatedAt(us))
	var usersWithRole []*users.UserWithRole
	for _, u := range us {
		usersWithRole = append(usersWithRole, &users.UserWithRole{User: *u, Role: *roles[u.ID]})
	}
	return usersWithRole, nil
}

// ListTeamUsers lists all the users in an team
func (d *DB) ListTeamUsers(ctx context.Context, teamID string) ([]*users.User, error) {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	return d.listTeamUsers(ctx, teamID, false)
}

// ListTeamMemberships lists all memberships of the database. Use with care.
func (d *DB) ListTeamMemberships(_ context.Context) ([]*users.TeamMembership, error) {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	var memberships []*users.TeamMembership
	for userID, membership := range d.teamMemberships {
		for teamID, roleID := range membership {
			memberships = append(memberships, &users.TeamMembership{
				UserID: userID,
				TeamID: teamID,
				RoleID: roleID,
			})
		}
	}
	return memberships, nil
}

func (d *DB) listTeamUsers(_ context.Context, teamID string, excludeNewUsers bool) ([]*users.User, error) {
	var us []*users.User
	for m, teamRoles := range d.teamMemberships {
		for tID := range teamRoles {
			if tID == teamID {
				u, err := d.findUserByID(m)
				if err != nil {
					return nil, err
				}
				if excludeNewUsers && u.FirstLoginAt.IsZero() {
					continue
				}
				us = append(us, u)
			}
		}
	}

	sort.Sort(usersByCreatedAt(us))
	return us, nil
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
		return nil, errors.New("team name cannot be blank")
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
func (d *DB) AddUserToTeam(_ context.Context, userID, teamID, roleID string) error {
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
	teamRoles[teamID] = roleID
	d.teamMemberships[userID] = teamRoles
	return nil
}

// CreateTeamAsUser creates a team from a name and sets user to be admin
func (d *DB) CreateTeamAsUser(ctx context.Context, name, userID string) (*users.Team, error) {
	team, err := d.CreateTeam(ctx, name)
	if err != nil {
		return nil, err
	}

	err = d.AddUserToTeam(ctx, userID, team.ID, "admin")
	if err != nil {
		return nil, err
	}

	return team, nil
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
	team, err := d.CreateTeamAsUser(ctx, teamName, userID)
	if err != nil {
		return nil, err
	}

	return team, nil
}

// RemoveUserFromTeam removes a user from a team
func (d *DB) RemoveUserFromTeam(ctx context.Context, userID, teamID string) error {

	delete(d.teamMemberships[userID], teamID)

	return nil
}
