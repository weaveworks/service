package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/Masterminds/squirrel"

	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/externalIDs"
)

// ListTeamsForUserID returns all teams belonging to userId
func (d DB) ListTeamsForUserID(ctx context.Context, userID string) ([]*users.Team, error) {
	query := d.teamQuery().
		Join("team_memberships m ON teams.id = m.team_id").
		Where(squirrel.Eq{"m.user_id": userID}).
		Where("m.deleted_at IS NULL")
	rows, err := query.Query()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return d.scanTeams(rows)
}

// ListTeamOrganizationsForUserIDs lists the organizations these users' teams belong to
func (d DB) ListTeamOrganizationsForUserIDs(_ context.Context, userIDs ...string) ([]*users.Organization, error) {
	rows, err := d.organizationsQuery().
		Join("team_memberships on (organizations.team_id = team_memberships.team_id)").
		Where("team_memberships.deleted_at IS NULL").
		Where(squirrel.Eq{"team_memberships.user_id": userIDs}).
		Query()
	if err != nil {
		return nil, err
	}
	orgs, err := d.scanOrganizations(rows)
	if err != nil {
		return nil, err
	}
	return orgs, err
}

// createTeam creates a team
func (d DB) createTeam(ctx context.Context) (*users.Team, error) {
	now := d.Now()
	TrialExpiresAt := now.Add(users.TrialDuration)
	t := &users.Team{TrialExpiresAt: TrialExpiresAt}

	err := d.Transaction(func(tx DB) error {
		externalID, err := tx.generateTeamExternalID(ctx)
		if err != nil {
			return err
		}
		t.ExternalID = externalID
		err = tx.QueryRow(`insert into teams (external_id, trial_expires_at)
						  values (lower($1), $2) returning id, created_at`, externalID, TrialExpiresAt).Scan(&t.ID, &t.CreatedAt)
		return err
	})
	if err != nil {
		return nil, err
	}
	return t, nil
}

// teamExternalIDUsed returns whether the team externalID has already been taken
func (d DB) teamExternalIDUsed(_ context.Context, externalID string) (bool, error) {
	var exists bool
	err := d.QueryRow(
		`select exists(select 1 from teams where external_id = lower($1))`,
		externalID,
	).Scan(&exists)
	return exists, err
}

// generateTeamExternalID generates a new team externalID.
// This function slows down the more externalIDs are stored in the database
func (d DB) generateTeamExternalID(ctx context.Context) (string, error) {
	var (
		externalID string
		err        error
		terr       error
	)
	err = d.Transaction(func(tx DB) error {
		for used := true; used; {
			externalID = externalIDs.Generate()
			used, terr = tx.teamExternalIDUsed(ctx, externalID)
			if terr != nil {
				return terr
			}
		}
		return nil
	})
	return externalID, err
}

// addUserToTeam links a user to the team
func (d DB) addUserToTeam(userID, teamID string) error {
	_, err := d.Exec(`
			insert into team_memberships
				(user_id, team_id)
				values ($1, $2)`,
		userID,
		teamID,
	)
	if err != nil {
		return err
	}
	return err
}

// removeUserFromTeam removes the user from the team.
// If they are not a team member, this is a noop.
func (d DB) removeUserFromTeam(userID, teamID string) error {
	_, err := d.Exec(
		"update team_memberships set deleted_at = now() where user_id = $1 and team_id = $2",
		userID,
		teamID,
	)
	return err
}

// setDefaultTeam sets a user's default team
func (d DB) setDefaultTeam(userID, teamID string) error {
	err := d.Transaction(func(tx DB) error {
		_, err := tx.Exec(
			"update team_memberships set is_default = NULL where user_id = $1",
			userID,
		)
		if err != nil {
			return err
		}
		_, err = tx.Exec(
			"update team_memberships set is_default = true where user_id = $1 and team_id = $2 and deleted_at is NULL",
			userID,
			teamID,
		)
		return err
	})
	return err
}

// defaultTeamByUserID returns the user's explicit default team or
func (d DB) defaultTeamByUserID(userID string) (*users.Team, error) {
	row := d.teamQuery().
		Join("team_memberships m ON teams.id = m.team_id").
		Where(squirrel.Eq{"m.user_id": userID}).
		Where("m.deleted_at IS NULL").
		OrderBy("m.is_default NULLS LAST").
		Limit(1).
		QueryRow()
	return d.scanTeam(row)
}

func (d DB) teamQuery() squirrel.SelectBuilder {
	return d.Select(`
		teams.id,
		teams.external_id,
		teams.zuora_account_number,
		teams.zuora_account_created_at,
		teams.trial_expires_at,
		teams.trial_pending_expiry_notified_at,
		teams.trial_expired_notified_at,
		teams.created_at
	`).
		From("teams").
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
		&t.ID, &t.ExternalID, &zuoraAccountNumber,
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
