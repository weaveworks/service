package postgres

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/lib/pq"

	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/externalids"
)

// ListTeamsForUserID returns all teams belonging to userId
func (d DB) ListTeamsForUserID(ctx context.Context, userID string) ([]*users.Team, error) {
	query := d.teamQuery().
		Join("team_memberships m ON teams.id = m.team_id").
		Where(squirrel.Eq{"m.user_id": userID}).
		Where("m.deleted_at IS NULL")
	rows, err := query.QueryContext(ctx)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return d.scanTeams(rows)
}

func (d DB) listTeamOrganizationsForUserIDs(ctx context.Context, userIDs ...string) ([]*users.Organization, error) {
	rows, err := d.organizationsQuery().
		Join("team_memberships on (organizations.team_id = team_memberships.team_id)").
		Where("team_memberships.deleted_at IS NULL").
		Where(squirrel.Eq{"team_memberships.user_id": userIDs}).
		QueryContext(ctx)
	if err != nil {
		return nil, err
	}
	orgs, err := d.scanOrganizations(rows)
	if err != nil {
		return nil, err
	}
	return orgs, err
}

func (d DB) teamHasOrganizations(ctx context.Context, teamID string) (bool, error) {
	var exists bool
	err := d.QueryRowContext(ctx,
		`select exists(
				select 1 from organizations join teams on teams.id = organizations.team_id
					where teams.id = $1 and teams.deleted_at is null and organizations.deleted_at is null
			)`,
		teamID,
	).Scan(&exists)
	return exists, err
}

// ListTeamUsers lists all the users in a team
func (d DB) ListTeamUsers(ctx context.Context, teamID string) ([]*users.User, error) {
	rows, err := d.usersQuery().
		Join("team_memberships on (team_memberships.user_id = users.id)").
		Where("team_memberships.deleted_at IS NULL").
		Where(squirrel.Eq{
			"team_memberships.team_id": teamID,
		}).
		QueryContext(ctx)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return d.scanUsers(rows)
}

// CreateTeam creates a team
func (d DB) CreateTeam(ctx context.Context, name string) (*users.Team, error) {
	if name == "" {
		return nil, errors.New("Team name cannot be blank")
	}
	now := d.Now()
	TrialExpiresAt := now.Add(users.TrialDuration)
	t := &users.Team{TrialExpiresAt: TrialExpiresAt}

	err := d.Transaction(func(tx DB) error {
		externalID, err := tx.generateTeamExternalID(ctx)
		if err != nil {
			return err
		}
		t.ExternalID = externalID
		err = tx.QueryRowContext(ctx, `insert into teams (external_id, trial_expires_at, name)
						  values (lower($1), $2, $3) returning id, created_at`, externalID, TrialExpiresAt, name).Scan(&t.ID, &t.CreatedAt)
		return err
	})
	if err != nil {
		return nil, err
	}
	return t, nil
}

// AddUserToTeam links a user to the team
func (d DB) AddUserToTeam(ctx context.Context, userID, teamID string) error {
	_, err := d.ExecContext(ctx, `
			insert into team_memberships
				(user_id, team_id)
				values ($1, $2)`,
		userID,
		teamID,
	)
	if err != nil {
		if e, ok := err.(*pq.Error); ok && e.Constraint == "team_memberships_user_id_team_id_idx" {
			return nil
		}
	}
	return err
}

// DeleteTeam marks the given team as deleted.
func (d DB) DeleteTeam(ctx context.Context, teamID string) error {
	// Verify team has no orgs
	has, err := d.teamHasOrganizations(ctx, teamID)
	if err != nil {
		return err
	}
	if has {
		return users.ErrForbidden
	}

	// Delete memberships
	if _, err := d.ExecContext(ctx,
		"update team_memberships set deleted_at = now() where team_id = $1",
		teamID,
	); err != nil {
		return err
	}

	// Delete team
	if _, err := d.ExecContext(ctx,
		"update teams set deleted_at = now() where id = $1",
		teamID,
	); err != nil {
		return err
	}

	return nil
}

// teamExternalIDUsed returns whether the team externalID has already been taken
func (d DB) teamExternalIDUsed(ctx context.Context, externalID string) (bool, error) {
	var exists bool
	err := d.QueryRowContext(ctx,
		`select exists(select 1 from teams where external_id = lower($1))`,
		externalID,
	).Scan(&exists)
	return exists, err
}

// generateTeamExternalID generates a new team externalID.
// This function slows down the more externalids are stored in the database
func (d DB) generateTeamExternalID(ctx context.Context) (string, error) {
	var (
		externalID string
		err        error
		terr       error
	)
	err = d.Transaction(func(tx DB) error {
		for used := true; used; {
			externalID = externalids.Generate()
			used, terr = tx.teamExternalIDUsed(ctx, externalID)
			if terr != nil {
				return terr
			}
		}
		return nil
	})
	return externalID, err
}

// removeUserFromTeam removes the user from the team.
// If they are not a team member, this is a noop.
func (d DB) removeUserFromTeam(ctx context.Context, userID, teamID string) error {
	_, err := d.ExecContext(ctx,
		"update team_memberships set deleted_at = now() where user_id = $1 and team_id = $2",
		userID,
		teamID,
	)
	return err
}

func (d DB) findTeamByExternalID(ctx context.Context, externalID string) (*users.Team, error) {
	team, err := d.scanTeam(
		d.teamQuery().Where("lower(teams.external_id) = lower($1)", externalID).QueryRowContext(ctx),
	)
	if err == sql.ErrNoRows {
		err = users.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return team, nil
}

func (d DB) teamQuery() squirrel.SelectBuilder {
	return d.Select(`
		teams.id,
		teams.external_id,
		teams.name,
		teams.zuora_account_number,
		teams.zuora_account_created_at,
		teams.trial_expires_at,
		teams.trial_pending_expiry_notified_at,
		teams.trial_expired_notified_at,
		teams.created_at
	`).
		From("teams").
		Where("teams.deleted_at is null").
		OrderBy("teams.created_at")
}

func (d DB) scanTeams(rows *sql.Rows) ([]*users.Team, error) {
	teams := []*users.Team{}
	for rows.Next() {
		team, err := d.scanTeam(rows)
		if err != nil {
			return nil, err
		}
		teams = append(teams, team)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return teams, nil
}

func (d DB) scanTeam(row squirrel.RowScanner) (*users.Team, error) {
	t := &users.Team{}
	var (
		zuoraAccountNumber sql.NullString

		zuoraAccountCreatedAt,
		trialPendingExpiryNotifiedAt,
		trialExpiredNotifiedAt *time.Time
	)
	if err := row.Scan(
		&t.ID, &t.ExternalID, &t.Name, &zuoraAccountNumber,
		&zuoraAccountCreatedAt, &t.TrialExpiresAt,
		&trialPendingExpiryNotifiedAt, &trialExpiredNotifiedAt,
		&t.CreatedAt,
	); err != nil {
		return nil, err
	}
	t.ZuoraAccountNumber = zuoraAccountNumber.String
	t.ZuoraAccountCreatedAt = zuoraAccountCreatedAt
	t.TrialPendingExpiryNotifiedAt = trialPendingExpiryNotifiedAt
	t.TrialExpiredNotifiedAt = trialExpiredNotifiedAt
	return t, nil
}

// getTeamUserIsPartOf ensures the users is part of an existing team
func (d DB) getTeamUserIsPartOf(ctx context.Context, userID, teamExternalID string) (*users.Team, error) {
	if teamExternalID == "" {
		return nil, errors.New("teamExternalID must be provided")
	}

	teams, err := d.ListTeamsForUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	for _, team := range teams {
		if team.ExternalID == teamExternalID {
			// user is already part of the team
			return team, nil
		}
	}

	return nil, users.ErrNotFound
}

// ensureUserIsPartOfTeamByName ensures the users is part of team by name, the team is created if it does not exist
func (d DB) ensureUserIsPartOfTeamByName(ctx context.Context, userID, teamName string) (*users.Team, error) {
	if teamName == "" {
		return nil, errors.New("teamName must be provided")
	}

	teams, err := d.ListTeamsForUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	for _, team := range teams {
		// case insensitive
		if strings.ToLower(team.Name) == strings.ToLower(teamName) {
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
