package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/lib/pq"

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

func (d DB) listTeamOrganizationsForUserIDs(_ context.Context, userIDs ...string) ([]*users.Organization, error) {
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

// ListTeamUsers lists all the users in a team
func (d DB) ListTeamUsers(ctx context.Context, teamID string) ([]*users.User, error) {
	rows, err := d.usersQuery().
		Join("team_memberships on (team_memberships.user_id = users.id)").
		Where("team_memberships.deleted_at IS NULL").
		Where(squirrel.Eq{
			"team_memberships.team_id": teamID,
		}).
		Query()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return d.scanUsers(rows)
}

// CreateTeam creates a team
func (d DB) CreateTeam(ctx context.Context, name string) (*users.Team, error) {
	now := d.Now()
	TrialExpiresAt := now.Add(users.TrialDuration)
	t := &users.Team{TrialExpiresAt: TrialExpiresAt}

	err := d.Transaction(func(tx DB) error {
		externalID, err := tx.generateTeamExternalID(ctx)
		if err != nil {
			return err
		}
		t.ExternalID = externalID
		err = tx.QueryRow(`insert into teams (external_id, trial_expires_at, name)
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
	_, err := d.Exec(`
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

func (d DB) findTeamByExternalID(_ context.Context, externalID string) (*users.Team, error) {
	team, err := d.scanTeam(
		d.teamQuery().Where("lower(teams.external_id) = lower($1)", externalID).QueryRow(),
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
