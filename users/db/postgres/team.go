package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/Masterminds/squirrel"

	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/externalIDs"
)

// CreateTeam creates a team and links the team to the userID
func (d DB) CreateTeam(ctx context.Context, userID string) (*users.Team, error) {
	now := d.Now()
	TrialExpiresAt := now.Add(users.TrialDuration)
	t := &users.Team{TrialExpiresAt: TrialExpiresAt}

	err := d.Transaction(func(tx DB) error {
		externalID, err := tx.GenerateTeamExternalID(ctx)
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

// TeamExternalIDUsed returns whether the team externalID has already been taken
func (d DB) TeamExternalIDUsed(_ context.Context, externalID string) (bool, error) {
	var exists bool
	err := d.QueryRow(
		`select exists(select 1 from teams where external_id = lower($1))`,
		externalID,
	).Scan(&exists)
	return exists, err
}

// GenerateTeamExternalID generates a new team externalID.
// This function slows down the more externalIDs are stored in the database
func (d DB) GenerateTeamExternalID(ctx context.Context) (string, error) {
	var (
		externalID string
		err        error
		terr       error
	)
	err = d.Transaction(func(tx DB) error {
		for used := true; used; {
			externalID = externalIDs.Generate()
			used, terr = tx.TeamExternalIDUsed(ctx, externalID)
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

// SetDefaultTeam sets a user's default team
func (d DB) SetDefaultTeam(userID, teamID string) error {
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

// DefaultTeamByUserID returns the user's explicit default team or
func (d DB) DefaultTeamByUserID(userID string) (*users.Team, error) {
	row := d.Select(`
		t.id,
		t.external_id,
		t.zuora_account_number,
		t.zuora_account_created_at,
		t.trial_expires_at,
		t.trial_pending_expiry_notified_at,
		t.trial_expired_notified_at,
		t.created_at
	`).
		From("teams t").
		Join("team_memberships m ON t.id = m.team_id").
		Where(squirrel.Eq{"m.user_id": userID}).
		Where("m.deleted_at IS NULL").
		OrderBy("m.is_default NULLS LAST").
		OrderBy("t.created_at").
		Limit(1).
		QueryRow()
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
