package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/lib/pq"

	"github.com/weaveworks/service/common/featureflag"
	timeutil "github.com/weaveworks/service/common/time"
	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/db/filter"
	"github.com/weaveworks/service/users/externalids"
)

// RemoveUserFromOrganization removes the user from the organiation. If they
// are not a member, this is a noop.
func (d DB) RemoveUserFromOrganization(ctx context.Context, orgExternalID, email string) error {
	err := d.Transaction(func(tx DB) error {
		user, err := tx.FindUserByEmail(ctx, email)
		if err != nil {
			return err
		}

		org, err := tx.FindOrganizationByID(ctx, orgExternalID)
		if err != nil {
			return err
		}

		_, err = tx.ExecContext(ctx,
			"update memberships set deleted_at = now() where user_id = $1 and organization_id = $2",
			user.ID,
			org.ID,
		)
		if err != nil {
			return err
		}

		return tx.removeUserFromTeam(ctx, user.ID, org.TeamID)
	})
	return err
}

// UserIsMemberOf checks if the user is a member of the organization.
// This includes checking membership and team membership
func (d DB) UserIsMemberOf(ctx context.Context, userID, orgExternalID string) (bool, error) {
	ok, err := d.userIsDirectMemberOf(ctx, userID, orgExternalID)
	if err != nil {
		return false, err
	}
	if ok {
		return ok, nil
	}
	ok, err = d.userIsTeamMemberOf(ctx, userID, orgExternalID)
	if err != nil {
		return false, err
	}
	return ok, nil
}

func (d DB) userIsDirectMemberOf(ctx context.Context, userID, orgExternalID string) (bool, error) {
	rows, err := d.organizationsQuery().
		Join("memberships on (organizations.id = memberships.organization_id)").
		Where(squirrel.Eq{"memberships.user_id": userID, "organizations.external_id": orgExternalID}).
		Where("memberships.deleted_at is null").
		QueryContext(ctx)
	if err != nil {
		return false, err
	}
	ok := rows.Next()
	if rows.Err() != nil {
		return false, rows.Err()
	}
	defer rows.Close()
	return ok, nil
}

func (d DB) userIsTeamMemberOf(ctx context.Context, userID, orgExternalID string) (bool, error) {
	rows, err := d.organizationsQuery().
		Join("team_memberships on (organizations.team_id = team_memberships.team_id)").
		Where(squirrel.Eq{
			"team_memberships.user_id":    userID,
			"team_memberships.deleted_at": nil,
			"organizations.external_id":   orgExternalID,
		}).
		QueryContext(ctx)
	if err != nil {
		return false, err
	}
	ok := rows.Next()
	if rows.Err() != nil {
		return false, rows.Err()
	}
	defer rows.Close()
	return ok, nil
}

func (d DB) organizationsQuery() squirrel.SelectBuilder {
	return d.organizationsQueryHelper(false)
}

func (d DB) organizationsQueryWithDeleted() squirrel.SelectBuilder {
	return d.organizationsQueryHelper(true)
}

func (d DB) organizationsQueryHelper(deleted bool) squirrel.SelectBuilder {
	query := d.Select(
		"organizations.id",
		"organizations.external_id",
		"organizations.name",
		"organizations.probe_token",
		"organizations.created_at",
		"organizations.deleted_at",
		"organizations.feature_flags",
		"organizations.refuse_data_access",
		"organizations.refuse_data_upload",
		"organizations.first_seen_connected_at",
		"organizations.platform",
		"organizations.environment",
		"organizations.zuora_account_number",
		"organizations.zuora_account_created_at",
		"organizations.trial_expires_at",
		"organizations.trial_pending_expiry_notified_at",
		"organizations.trial_expired_notified_at",
		"organizations.gcp_account_id",
		"gcp_accounts.created_at",
		"gcp_accounts.external_account_id",
		"gcp_accounts.activated",
		"gcp_accounts.consumer_id",
		"gcp_accounts.subscription_name",
		"gcp_accounts.subscription_level",
		"gcp_accounts.subscription_status",
		"organizations.team_id",
		"teams.external_id",
		"organizations.first_seen_flux_connected_at",
		"organizations.first_seen_net_connected_at",
		"organizations.first_seen_prom_connected_at",
		"organizations.first_seen_scope_connected_at",
	).
		From("organizations").
		LeftJoin("gcp_accounts ON gcp_account_id = gcp_accounts.id").
		LeftJoin("teams ON teams.id = organizations.team_id AND teams.deleted_at is null").
		OrderBy("organizations.created_at DESC")
	if !deleted {
		query = query.Where("organizations.deleted_at is null")
	}
	return query
}

// ListOrganizations lists organizations
func (d DB) ListOrganizations(ctx context.Context, f filter.Organization, page uint64) ([]*users.Organization, error) {
	q := d.organizationsQuery().Where(f.Where())
	if page > 0 {
		q = q.Limit(filter.ResultsPerPage).Offset((page - 1) * filter.ResultsPerPage)
	}

	rows, err := q.QueryContext(ctx)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return d.scanOrganizations(rows)
}

// ListAllOrganizations lists all organizations including deleted ones
func (d DB) ListAllOrganizations(ctx context.Context, f filter.Organization, page uint64) ([]*users.Organization, error) {
	// check that query is not empty, otherwise q might be like "... where order by ..."
	queryStr, _, err := f.Where().ToSql()
	if err != nil {
		return nil, err
	}
	q := d.organizationsQueryWithDeleted()
	if queryStr != "" {
		q = q.Where(f.Where())
	}

	if page > 0 {
		q = q.Limit(filter.ResultsPerPage).Offset((page - 1) * filter.ResultsPerPage)
	}

	rows, err := q.QueryContext(ctx)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return d.scanOrganizations(rows)
}

// ListOrganizationUsers lists all the users in an organization
func (d DB) ListOrganizationUsers(ctx context.Context, orgExternalID string) ([]*users.User, error) {
	orgUsers, err := d.listDirectOrganizationUsers(ctx, orgExternalID)
	if err != nil {
		return nil, err
	}
	teamUsers, err := d.listTeamOrganizationUsers(ctx, orgExternalID)
	if err != nil {
		return nil, err
	}
	users := mergeUsers(orgUsers, teamUsers)
	sort.Sort(usersByCreatedAt(users))
	return users, nil
}

func (d DB) listDirectOrganizationUsers(ctx context.Context, orgExternalID string) ([]*users.User, error) {
	rows, err := d.usersQuery().
		Join("memberships on (memberships.user_id = users.id)").
		Join("organizations on (memberships.organization_id = organizations.id)").
		Where(squirrel.Eq{
			"organizations.external_id": orgExternalID,
			"memberships.deleted_at":    nil,
			"organizations.deleted_at":  nil,
		}).
		OrderBy("users.created_at").
		QueryContext(ctx)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return d.scanUsers(rows)
}

func (d DB) listTeamOrganizationUsers(ctx context.Context, orgExternalID string) ([]*users.User, error) {
	rows, err := d.usersQuery().
		Join("team_memberships on (team_memberships.user_id = users.id)").
		Join("organizations on (team_memberships.team_id = organizations.team_id)").
		Where(squirrel.Eq{
			"organizations.external_id":   orgExternalID,
			"organizations.deleted_at":    nil,
			"team_memberships.deleted_at": nil,
		}).
		QueryContext(ctx)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return d.scanUsers(rows)
}

// ListOrganizationsForUserIDs lists the organizations these users belong to.
// This includes direct membership and team membership.
func (d DB) ListOrganizationsForUserIDs(ctx context.Context, userIDs ...string) ([]*users.Organization, error) {
	// SQL UNIONs are not supported by github.com/Masterminds/squirrel
	memberOrgs, err := d.listMemberOrganizationsForUserIDs(ctx, userIDs...)
	if err != nil {
		return nil, err
	}
	teamOrgs, err := d.listTeamOrganizationsForUserIDs(ctx, userIDs...)
	if err != nil {
		return nil, err
	}
	orgs := mergeOrgs(memberOrgs, teamOrgs)
	sort.Sort(organizationsByCreatedAt(orgs))
	return orgs, nil
}

// listMemberOrganizationsForUserIDs lists the organizations these users belong to
func (d DB) listMemberOrganizationsForUserIDs(ctx context.Context, userIDs ...string) ([]*users.Organization, error) {
	rows, err := d.organizationsQuery().
		Join("memberships on (organizations.id = memberships.organization_id)").
		Where(squirrel.Eq{"memberships.user_id": userIDs}).
		Where("memberships.deleted_at is null").
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

// addUserToOrganization adds a user to the team of the organization,
// if the organization belongs to a team, otherwise populates membership
func (d DB) addUserToOrganization(ctx context.Context, userID, orgID string) error {
	_, err := d.ExecContext(ctx, `
			insert into memberships
				(user_id, organization_id, created_at)
				values ($1, $2, $3)`,
		userID,
		orgID,
		d.Now(),
	)
	if err != nil {
		if e, ok := err.(*pq.Error); ok && e.Constraint == "memberships_user_id_organization_id_idx" {
			return nil
		}
	}
	return err
}

// GenerateOrganizationExternalID returns an available organization external
// id, e.g. creaky-door-97
// TODO: There is a known issue, where as we fill up the database this will
// gradually slow down (since the algorithm is quite naive). We should fix it
// eventually.
func (d DB) GenerateOrganizationExternalID(ctx context.Context) (string, error) {
	var (
		externalID string
		err        error
		terr       error
	)
	err = d.Transaction(func(tx DB) error {
		for used := true; used; {
			externalID = externalids.Generate()
			used, terr = tx.ExternalIDUsed(ctx, externalID)
			if terr != nil {
				return terr
			}
		}
		return nil
	})
	return externalID, err
}

// CreateOrganization creates a new organization owned by the user
func (d DB) CreateOrganization(ctx context.Context, ownerID, externalID, name, token, teamID string, trialExpiresAt time.Time) (*users.Organization, error) {
	now := d.Now()
	o := &users.Organization{
		ExternalID:     externalID,
		Name:           name,
		CreatedAt:      now,
		TrialExpiresAt: trialExpiresAt,
		TeamID:         teamID,
	}
	if err := o.Valid(); err != nil {
		return nil, err
	}

	err := d.Transaction(func(tx DB) error {
		used, err := tx.ExternalIDUsed(ctx, o.ExternalID)
		if err != nil {
			return err
		}
		if used {
			return users.ErrOrgExternalIDIsTaken
		}

		for exists := true; exists; {
			if token != "" {
				o.ProbeToken = token
			} else {
				if err := o.RegenerateProbeToken(); err != nil {
					return err
				}
			}

			if err := tx.QueryRowContext(ctx,
				`select exists(select 1 from organizations where probe_token = $1 and deleted_at is null)`,
				o.ProbeToken,
			).Scan(&exists); err != nil {
				return err
			}
			if token != "" && exists {
				return users.ErrOrgTokenIsTaken
			}
		}

		err = tx.QueryRowContext(ctx, `insert into organizations
			(external_id, name, probe_token, created_at, trial_expires_at, team_id)
			values (lower($1), $2, $3, $4, $5, $6) returning id`,
			o.ExternalID, o.Name, o.ProbeToken, o.CreatedAt, o.TrialExpiresAt, toNullString(o.TeamID),
		).Scan(&o.ID)
		if err != nil {
			return err
		}

		if o.TeamID == "" {
			return tx.addUserToOrganization(ctx, ownerID, o.ID)
		}

		return err
	})
	if err != nil {
		return nil, err
	}
	return o, err
}

// FindUncleanedOrgIDs looks up deleted but uncleaned organization IDs
func (d DB) FindUncleanedOrgIDs(ctx context.Context) ([]string, error) {
	rows, err := d.Select("organizations.id").
		From("organizations").
		Where(squirrel.Expr("organizations.cleanup = false and (organizations.deleted_at is not null or organizations.refuse_data_upload = true)")).
		QueryContext(ctx)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return ids, nil
}

// FindOrganizationByProbeToken looks up the organization matching a given
// probe token.
func (d DB) FindOrganizationByProbeToken(ctx context.Context, probeToken string) (*users.Organization, error) {
	o, err := d.scanOrganization(
		d.organizationsQuery().Where(squirrel.Eq{"organizations.probe_token": probeToken}).QueryRowContext(ctx),
	)
	if err == sql.ErrNoRows {
		err = users.ErrNotFound
	}
	if err != nil {
		o = nil
	}
	return o, err
}

// FindOrganizationByID looks up the organization matching a given
// external ID.
func (d DB) FindOrganizationByID(ctx context.Context, externalID string) (*users.Organization, error) {
	o, err := d.scanOrganization(
		d.organizationsQuery().Where(squirrel.Eq{"organizations.external_id": externalID}).QueryRowContext(ctx),
	)
	if err == sql.ErrNoRows {
		return nil, users.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return o, nil
}

// FindOrganizationByGCPExternalAccountID returns the organization with the given account ID.
// N.B.: it only returns GCP organizations which have been activated, i.e. for which the subscription has been validated and activated against GCP.
func (d DB) FindOrganizationByGCPExternalAccountID(ctx context.Context, externalAccountID string) (*users.Organization, error) {
	gcp, err := d.FindGCP(ctx, externalAccountID)
	if err != nil {
		return nil, err
	}
	o, err := d.scanOrganization(
		d.organizationsQuery().Where(squirrel.Eq{"organizations.gcp_account_id": gcp.ID}).QueryRowContext(ctx),
	)
	if err != nil {
		// If error is sql.ErrNoRows we have a dangling GCP account ID.
		return nil, err
	}
	return o, nil
}

// FindOrganizationByInternalID finds an org based on its ID
func (d DB) FindOrganizationByInternalID(ctx context.Context, internalID string) (*users.Organization, error) {
	o, err := d.scanOrganization(
		d.organizationsQuery().Where(squirrel.Eq{"organizations.id": internalID}).QueryRowContext(ctx),
	)

	if err == sql.ErrNoRows {
		return nil, users.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return o, nil
}

func (d DB) scanOrganizations(rows *sql.Rows) ([]*users.Organization, error) {
	orgs := []*users.Organization{}
	for rows.Next() {
		org, err := d.scanOrganization(rows)
		if err != nil {
			return nil, err
		}
		orgs = append(orgs, org)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return orgs, nil
}

func (d DB) scanOrganization(row squirrel.RowScanner) (*users.Organization, error) {
	o := &users.Organization{}
	var externalID, name, probeToken, platform, environment, zuoraAccountNumber, teamID, teamExternalID sql.NullString
	var createdAt pq.NullTime
	var deletedAt pq.NullTime
	var trialExpiry time.Time
	var trialExpiredNotifiedAt, trialPendingExpiryNotifiedAt *time.Time
	var refuseDataAccess, refuseDataUpload bool
	var gcpID, externalAccountID, consumerID, subscriptionName, subscriptionLevel, subscriptionStatus sql.NullString
	var gcpCreatedAt pq.NullTime
	var activated sql.NullBool
	if err := row.Scan(
		&o.ID,
		&externalID,
		&name,
		&probeToken,
		&createdAt,
		&deletedAt,
		pq.Array(&o.FeatureFlags),
		&refuseDataAccess,
		&refuseDataUpload,
		&o.FirstSeenConnectedAt,
		&platform,
		&environment,
		&zuoraAccountNumber,
		&o.ZuoraAccountCreatedAt,
		&trialExpiry,
		&trialPendingExpiryNotifiedAt,
		&trialExpiredNotifiedAt,
		&gcpID,
		&gcpCreatedAt,
		&externalAccountID,
		&activated,
		&consumerID,
		&subscriptionName,
		&subscriptionLevel,
		&subscriptionStatus,
		&teamID,
		&teamExternalID,
		&o.FirstSeenFluxConnectedAt,
		&o.FirstSeenNetConnectedAt,
		&o.FirstSeenPromConnectedAt,
		&o.FirstSeenScopeConnectedAt,
	); err != nil {
		return nil, err
	}
	o.ExternalID = externalID.String
	o.Name = name.String
	o.ProbeToken = probeToken.String
	o.CreatedAt = createdAt.Time
	o.DeletedAt = deletedAt.Time
	o.RefuseDataAccess = refuseDataAccess
	o.RefuseDataUpload = refuseDataUpload
	o.Platform = platform.String
	o.Environment = environment.String
	o.ZuoraAccountNumber = zuoraAccountNumber.String
	o.TrialExpiresAt = trialExpiry
	o.TrialPendingExpiryNotifiedAt = trialPendingExpiryNotifiedAt
	o.TrialExpiredNotifiedAt = trialExpiredNotifiedAt
	if gcpID.Valid {
		o.GCP = &users.GoogleCloudPlatform{
			ID:                 gcpID.String,
			CreatedAt:          gcpCreatedAt.Time,
			ExternalAccountID:  externalAccountID.String,
			Activated:          activated.Bool,
			ConsumerID:         consumerID.String,
			SubscriptionName:   subscriptionName.String,
			SubscriptionLevel:  subscriptionLevel.String,
			SubscriptionStatus: subscriptionStatus.String,
		}
	}
	o.TeamID = teamID.String
	o.TeamExternalID = teamExternalID.String
	return o, nil
}

// UpdateOrganization changes an organization's user-settable name
func (d DB) UpdateOrganization(ctx context.Context, externalID string, update users.OrgWriteView) (*users.Organization, error) {
	// Get org for validation and add update fields to setFields
	org, err := d.FindOrganizationByID(ctx, externalID)
	if err != nil {
		return nil, err
	}
	setFields := map[string]interface{}{}
	if update.Name != nil {
		org.Name = *update.Name
		setFields["name"] = *update.Name
	}
	if update.Platform != nil {
		org.Platform = *update.Platform
		setFields["platform"] = *update.Platform
	}
	if update.Environment != nil {
		org.Environment = *update.Environment
		setFields["environment"] = *update.Environment
	}
	if update.TrialExpiresAt != nil {
		org.TrialExpiresAt = *update.TrialExpiresAt
		setFields["trial_expires_at"] = *update.TrialExpiresAt
	}
	if update.TrialPendingExpiryNotifiedAt != nil {
		org.TrialPendingExpiryNotifiedAt = timeutil.ZeroTimeIsNil(update.TrialPendingExpiryNotifiedAt)
		setFields["trial_pending_expiry_notified_at"] = org.TrialPendingExpiryNotifiedAt
	}
	if update.TrialExpiredNotifiedAt != nil {
		org.TrialExpiredNotifiedAt = timeutil.ZeroTimeIsNil(update.TrialExpiredNotifiedAt)
		setFields["trial_expired_notified_at"] = org.TrialExpiredNotifiedAt
	}

	if len(setFields) == 0 {
		return org, nil
	}

	if err := org.Valid(); err != nil {
		return nil, err
	}

	result, err := d.Update("organizations").
		SetMap(setFields).
		Where(squirrel.Expr("external_id = lower(?) and deleted_at is null", externalID)).
		ExecContext(ctx)
	if err != nil {
		return nil, err
	}
	count, err := result.RowsAffected()
	switch {
	case err != nil:
		return nil, err
	case count != 1:
		return nil, users.ErrNotFound
	}
	return org, nil
}

// MoveOrganizationToTeam updates the team of the organization. It does *not* check team permissions.
func (d DB) MoveOrganizationToTeam(ctx context.Context, externalID, teamExternalID, teamName, userID string) error {
	var team *users.Team
	var err error
	if teamName != "" {
		if team, err = d.CreateTeam(ctx, teamName); err != nil {
			return err
		}
		if err = d.AddUserToTeam(ctx, userID, team.ID); err != nil {
			return err
		}
	} else {
		if team, err = d.findTeamByExternalID(ctx, teamExternalID); err != nil {
			return err
		}
	}
	_, err = d.ExecContext(ctx, `update organizations set team_id = $1 where external_id = lower($2)`,
		team.ID, externalID)
	return err
}

// OrganizationExists just returns a simple bool checking if an organization
// exists. It exists if it hasn't been deleted.
func (d DB) OrganizationExists(ctx context.Context, externalID string) (bool, error) {
	var exists bool
	err := d.QueryRowContext(ctx,
		`select exists(select 1 from organizations where external_id = lower($1) and deleted_at is null)`,
		externalID,
	).Scan(&exists)
	return exists, err
}

// ExternalIDUsed returns true if the given `externalID` has ever been in use for
// an organization.
func (d DB) ExternalIDUsed(ctx context.Context, externalID string) (bool, error) {
	var exists bool
	err := d.QueryRowContext(ctx,
		`select exists(select 1 from organizations where external_id = lower($1))`,
		externalID,
	).Scan(&exists)
	return exists, err
}

// GetOrganizationName gets the name of an organization from its external ID.
func (d DB) GetOrganizationName(ctx context.Context, externalID string) (string, error) {
	var name string
	err := d.QueryRowContext(ctx,
		`select name from organizations where external_id = lower($1) and deleted_at is null`,
		externalID,
	).Scan(&name)
	return name, err
}

// DeleteOrganization deletes an organization
func (d DB) DeleteOrganization(ctx context.Context, externalID string) error {
	_, err := d.ExecContext(ctx,
		`update organizations set deleted_at = $1 where external_id = lower($2) and deleted_at is null`,
		d.Now(), externalID,
	)
	return err
}

// AddFeatureFlag adds a new feature flag to a organization.
func (d DB) AddFeatureFlag(ctx context.Context, externalID string, featureFlag string) error {
	_, err := d.ExecContext(ctx,
		`update organizations set feature_flags = feature_flags || $1 where external_id = lower($2) and deleted_at is null`,
		pq.Array([]string{featureFlag}), externalID,
	)
	return err
}

// SetFeatureFlags sets all feature flags of an organization.
func (d DB) SetFeatureFlags(ctx context.Context, externalID string, featureFlags []string) error {
	if featureFlags == nil {
		featureFlags = []string{}
	}
	_, err := d.ExecContext(ctx,
		`update organizations set feature_flags = $1 where external_id = lower($2) and deleted_at is null`,
		pq.Array(featureFlags), externalID,
	)
	return err
}

// SetOrganizationCleanup sets cleanup true for organization with internalID
func (d DB) SetOrganizationCleanup(ctx context.Context, internalID string, value bool) error {
	_, err := d.ExecContext(ctx,
		`update organizations set cleanup = $1 where id = lower($2)`,
		value, internalID,
	)
	return err
}

// SetOrganizationRefuseDataAccess sets the "deny UI features" flag on an organization
func (d DB) SetOrganizationRefuseDataAccess(ctx context.Context, externalID string, value bool) error {
	_, err := d.ExecContext(ctx,
		`update organizations set refuse_data_access = $1 where external_id = lower($2) and deleted_at is null`,
		value, externalID,
	)
	return err
}

// SetOrganizationRefuseDataUpload sets the "deny token auth" flag on an organization
func (d DB) SetOrganizationRefuseDataUpload(ctx context.Context, externalID string, value bool) error {
	_, err := d.ExecContext(ctx,
		`update organizations set refuse_data_upload = $1 where external_id = lower($2) and deleted_at is null`,
		value, externalID,
	)
	return err
}

// SetOrganizationFirstSeenConnectedAt sets the first time an organisation has been connected
func (d DB) SetOrganizationFirstSeenConnectedAt(ctx context.Context, externalID string, value *time.Time) error {
	_, err := d.ExecContext(ctx,
		`update organizations set first_seen_connected_at = $1 where external_id = lower($2) and deleted_at is null`,
		value, externalID,
	)
	return err
}

func (d DB) setOrganizationAgentFirstSeenConnectedAt(ctx context.Context, externalID, agent string, value *time.Time) error {
	_, err := d.Update("organizations").
		Where(squirrel.Expr("external_id = lower(?) and deleted_at is null", externalID)).
		Set(fmt.Sprintf("first_seen_%s_connected_at", agent), value).
		ExecContext(ctx)
	return err
}

// SetOrganizationFirstSeenFluxConnectedAt sets the first time an organisation flux agent has been connected
func (d DB) SetOrganizationFirstSeenFluxConnectedAt(ctx context.Context, externalID string, value *time.Time) error {
	return d.setOrganizationAgentFirstSeenConnectedAt(ctx, externalID, "flux", value)
}

// SetOrganizationFirstSeenNetConnectedAt sets the first time an organisation weave net agent has been connected
func (d DB) SetOrganizationFirstSeenNetConnectedAt(ctx context.Context, externalID string, value *time.Time) error {
	return d.setOrganizationAgentFirstSeenConnectedAt(ctx, externalID, "net", value)
}

// SetOrganizationFirstSeenPromConnectedAt sets the first time an organisation prometheus agent has been connected
func (d DB) SetOrganizationFirstSeenPromConnectedAt(ctx context.Context, externalID string, value *time.Time) error {
	return d.setOrganizationAgentFirstSeenConnectedAt(ctx, externalID, "prom", value)
}

// SetOrganizationFirstSeenScopeConnectedAt sets the first time an organisation scope agent has been connected
func (d DB) SetOrganizationFirstSeenScopeConnectedAt(ctx context.Context, externalID string, value *time.Time) error {
	return d.setOrganizationAgentFirstSeenConnectedAt(ctx, externalID, "scope", value)
}

// SetOrganizationZuoraAccount sets the account number and time it was created at.
func (d DB) SetOrganizationZuoraAccount(ctx context.Context, externalID, number string, createdAt *time.Time) error {
	_, err := d.ExecContext(ctx,
		`update organizations set zuora_account_number = $1, zuora_account_created_at = $2 where external_id = lower($3) and deleted_at is null`,
		number, createdAt, externalID,
	)
	return err
}

// CreateOrganizationWithGCP creates an organization with an inactive GCP account attached to it.
func (d DB) CreateOrganizationWithGCP(ctx context.Context, ownerID, externalAccountID string, trialExpiresAt time.Time) (*users.Organization, error) {
	var org *users.Organization
	var gcp *users.GoogleCloudPlatform
	err := d.Transaction(func(tx DB) error {
		externalID, err := tx.GenerateOrganizationExternalID(ctx)
		if err != nil {
			return err
		}
		name := users.DefaultOrganizationName(externalID)
		// create one team for each gcp instance
		teamName := users.DefaultTeamName(externalID)
		org, err = tx.CreateOrganizationWithTeam(ctx, ownerID, externalID, name, "", "", teamName, trialExpiresAt)
		if err != nil {
			return err
		}

		// Create and attach inactive GCP subscription to the organization
		gcp, err = tx.createGCP(ctx, externalAccountID)
		if err != nil {
			return err
		}

		err = tx.SetOrganizationGCP(ctx, externalID, externalAccountID)
		return err

	})
	if err != nil {
		return nil, err
	}

	org.GCP = gcp
	return org, nil
}

// FindGCP returns the Google Cloud Platform subscription for the given account.
func (d DB) FindGCP(ctx context.Context, externalAccountID string) (*users.GoogleCloudPlatform, error) {
	var gcp users.GoogleCloudPlatform
	var consumerID, name, level, status sql.NullString
	err := d.QueryRowContext(ctx,
		`select id, external_account_id, activated, created_at, consumer_id, subscription_name, subscription_level, subscription_status
		from gcp_accounts
		where external_account_id = $1`,
		externalAccountID,
	).Scan(&gcp.ID, &gcp.ExternalAccountID, &gcp.Activated, &gcp.CreatedAt, &consumerID, &name, &level, &status)
	if err == sql.ErrNoRows {
		return nil, users.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	gcp.ConsumerID = consumerID.String
	gcp.SubscriptionName = name.String
	gcp.SubscriptionLevel = level.String
	gcp.SubscriptionStatus = status.String
	return &gcp, nil
}

// UpdateGCP Update a Google Cloud Platform entry. This marks the account as activated.
func (d DB) UpdateGCP(ctx context.Context, externalAccountID, consumerID, subscriptionName, subscriptionLevel, subscriptionStatus string) error {
	_, err := d.ExecContext(ctx,
		`update gcp_accounts
		set activated = true, consumer_id = $2, subscription_name = $3, subscription_level = $4, subscription_status = $5
		where external_account_id = $1`,
		externalAccountID, consumerID, subscriptionName, subscriptionLevel, subscriptionStatus,
	)
	return err
}

// SetOrganizationGCP attaches a Google Cloud Platform subscription to an organization.
// It also enables the billing feature flag and sets platform/env.
func (d DB) SetOrganizationGCP(ctx context.Context, externalID, externalAccountID string) error {
	return d.Transaction(func(tx DB) error {
		gcp, err := d.FindGCP(ctx, externalAccountID)
		if err != nil {
			return err
		}

		_, err = d.ExecContext(ctx,
			`update organizations set gcp_account_id = $1 where external_id = $2 and deleted_at is null`,
			gcp.ID, externalID,
		)
		if err != nil {
			return err
		}

		platform, env := "kubernetes", "gke"
		now := d.Now()
		if _, err = tx.UpdateOrganization(ctx, externalID, users.OrgWriteView{
			// Hardcode platform/env here, that's what we expect the user to have.
			// It also skips the platform/env tab during the onboarding process.
			Platform:    &platform,
			Environment: &env,
			// No trial for GCP instances
			TrialExpiresAt: &now,
		}); err != nil {
			return err
		}

		return tx.AddFeatureFlag(ctx, externalID, featureflag.Billing)
	})
}

// CreateOrganizationWithTeam creates a new organization, ensuring it is part of a team and owned by the user
func (d DB) CreateOrganizationWithTeam(ctx context.Context, ownerID, externalID, name, token, teamExternalID, teamName string, trialExpiresAt time.Time) (*users.Organization, error) {
	var org *users.Organization
	err := d.Transaction(func(tx DB) error {
		var team *users.Team
		var err error
		// one of two cases must be reached: it is ensured by the validation above
		if teamExternalID != "" {
			team, err = d.getTeamUserIsPartOf(ctx, ownerID, teamExternalID)
		} else if teamName != "" {
			team, err = d.ensureUserIsPartOfTeamByName(ctx, ownerID, teamName)
		}
		if err != nil {
			return err
		}

		// it should not happen, but just in case, be loud
		if team == nil {
			return fmt.Errorf("team should not be nil: %v, %v", teamExternalID, teamName)
		}

		org, err = tx.CreateOrganization(ctx, ownerID, externalID, name, token, team.ID, trialExpiresAt)
		return err
	})
	if err != nil {
		return nil, err
	}

	return org, nil
}

// createGCP creates a Google Cloud Platform account/subscription. It is initialized as inactive.
func (d DB) createGCP(ctx context.Context, externalAccountID string) (*users.GoogleCloudPlatform, error) {
	now := d.Now()
	gcp := &users.GoogleCloudPlatform{
		ExternalAccountID: externalAccountID,
		CreatedAt:         now,
	}
	err := d.QueryRowContext(ctx, `insert into gcp_accounts
			(external_account_id, created_at, activated)
			values ($1, $2, false) returning id`,
		gcp.ExternalAccountID, gcp.CreatedAt).
		Scan(&gcp.ID)
	if err != nil {
		return nil, err
	}

	return gcp, nil
}

func mergeOrgs(orgsSlice ...[]*users.Organization) []*users.Organization {
	m := make(map[string]*users.Organization)
	for _, orgs := range orgsSlice {
		for _, org := range orgs {
			if _, exists := m[org.ID]; !exists {
				m[org.ID] = org
			}
		}
	}
	uniqueOrgs := make([]*users.Organization, 0, len(m))
	for _, org := range m {
		uniqueOrgs = append(uniqueOrgs, org)
	}
	return uniqueOrgs
}

func mergeUsers(usersSlice ...[]*users.User) []*users.User {
	m := make(map[string]*users.User)
	for _, users := range usersSlice {
		for _, user := range users {
			if _, exists := m[user.ID]; !exists {
				m[user.ID] = user
			}
		}
	}
	uniqueUsers := make([]*users.User, 0, len(m))
	for _, user := range m {
		uniqueUsers = append(uniqueUsers, user)
	}
	return uniqueUsers
}

type organizationsByCreatedAt []*users.Organization

func (o organizationsByCreatedAt) Len() int           { return len(o) }
func (o organizationsByCreatedAt) Swap(i, j int)      { o[i], o[j] = o[j], o[i] }
func (o organizationsByCreatedAt) Less(i, j int) bool { return o[i].CreatedAt.After(o[j].CreatedAt) }

func toNullString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

// GetSummary exports a summary of the DB.
// WARNING: this is a relatively expensive query, and basically exports the entire DB.
// DISCLAIMER: In no event shall the authors be liable for any eye-bleed occurring
//             while reading the below SQL query. Sincere apologies, though.
func (d DB) GetSummary(ctx context.Context) ([]*users.SummaryEntry, error) {
	rows, err := d.QueryContext(ctx, `
		SELECT team_external_id, team_name, org_id, org_external_id, org_name,
		ARRAY_AGG(DISTINCT email ORDER BY email) AS emails,
		org_created_at, first_seen_connected_at, platform, environment,
		trial_expires_at, trial_pending_expiry_notified_at, trial_expired_notified_at,
		billing_enabled, refuse_data_access, refuse_data_upload,
		zuora_account_number, zuora_account_created_at,
		gcp_account_external_id, gcp_account_created_at, gcp_account_status, gcp_account_plan

		FROM (
		SELECT
		teams.external_id AS team_external_id,
		teams.name AS team_name,
		organizations.id AS org_id,
		organizations.external_id AS org_external_id,
		organizations.name AS org_name,
		users.email,
		organizations.created_at AS org_created_at,
		organizations.first_seen_connected_at,
		organizations.platform,
		organizations.environment,
		organizations.trial_expires_at,
		organizations.trial_pending_expiry_notified_at,
		organizations.trial_expired_notified_at,
		CASE WHEN array_position(organizations.feature_flags, 'billing') IS NULL THEN false ELSE true END AS billing_enabled,
		organizations.refuse_data_access,
		organizations.refuse_data_upload,
		organizations.zuora_account_number,
		organizations.zuora_account_created_at,
		gcp_accounts.external_account_id AS gcp_account_external_id,
		gcp_accounts.created_at AS gcp_account_created_at,
		gcp_accounts.subscription_status AS gcp_account_status,
		gcp_accounts.subscription_level AS gcp_account_plan
		FROM organizations
		LEFT JOIN teams        ON teams.id = organizations.team_id AND teams.deleted_at IS NULL
		LEFT JOIN gcp_accounts ON gcp_accounts.id = organizations.gcp_account_id
		INNER JOIN memberships  ON memberships.organization_id = organizations.id
		INNER JOIN users        ON users.id = memberships.user_id
		WHERE
		users.deleted_at IS NULL AND
		memberships.deleted_at IS NULL AND
		organizations.deleted_at IS NULL

		UNION

		SELECT
		teams.external_id AS team_external_id,
		teams.name AS team_name,
		organizations.id AS org_id,
		organizations.external_id AS org_external_id,
		organizations.name AS org_name,
		users.email,
		organizations.created_at AS org_created_at,
		organizations.first_seen_connected_at,
		organizations.platform,
		organizations.environment,
		organizations.trial_expires_at,
		organizations.trial_pending_expiry_notified_at,
		organizations.trial_expired_notified_at,
		CASE WHEN array_position(organizations.feature_flags, 'billing') IS NULL THEN false ELSE true END AS billing_enabled,
		organizations.refuse_data_access,
		organizations.refuse_data_upload,
		organizations.zuora_account_number,
		organizations.zuora_account_created_at,
		gcp_accounts.external_account_id AS gcp_account_external_id,
		gcp_accounts.created_at AS gcp_account_created_at,
		gcp_accounts.subscription_status AS gcp_account_status,
		gcp_accounts.subscription_level AS gcp_account_plan
		FROM organizations
		LEFT JOIN teams             ON teams.id = organizations.team_id
		LEFT JOIN gcp_accounts      ON gcp_accounts.id = organizations.gcp_account_id
		INNER JOIN team_memberships ON team_memberships.team_id = organizations.team_id
		INNER JOIN users            ON users.id = team_memberships.user_id
		WHERE
		users.deleted_at IS NULL AND
		team_memberships.deleted_at IS NULL AND
		organizations.deleted_at IS NULL AND
		teams.deleted_at IS NULL
		

		) AS t

		GROUP BY team_external_id, team_name, org_id, org_external_id, org_name, org_created_at, first_seen_connected_at, platform, environment,
		trial_expires_at, trial_pending_expiry_notified_at, trial_expired_notified_at,
		billing_enabled, refuse_data_access, refuse_data_upload,
		zuora_account_number, zuora_account_created_at,
		gcp_account_external_id, gcp_account_created_at, gcp_account_status, gcp_account_plan
		ORDER BY org_created_at DESC;`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return d.scanSummaryEntries(rows)
}

func (d DB) scanSummaryEntries(rows *sql.Rows) ([]*users.SummaryEntry, error) {
	entries := []*users.SummaryEntry{}
	for rows.Next() {
		entry, err := d.scanSummaryEntry(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return entries, nil
}

func (d DB) scanSummaryEntry(row squirrel.RowScanner) (*users.SummaryEntry, error) {
	e := &users.SummaryEntry{}
	var teamExternalID, teamName, orgID, orgExternalID, orgName, platform, environment, zuoraAccountNumber,
		gcpAccountExternalID, gcpAccountSubscriptionStatus, gcpAccountSubscriptionlevel sql.NullString
	var orgCreatedAt, gcpAccountCreatedAt pq.NullTime

	if err := row.Scan(
		&teamExternalID, &teamName, &orgID, &orgExternalID, &orgName, pq.Array(&e.Emails), &orgCreatedAt, &e.FirstSeenConnectedAt,
		&platform, &environment, &e.TrialExpiresAt, &e.TrialPendingExpiryNotifiedAt, &e.TrialExpiredNotifiedAt,
		&e.BillingEnabled, &e.RefuseDataAccess, &e.RefuseDataUpload, &zuoraAccountNumber, &e.ZuoraAccountCreatedAt,
		&gcpAccountExternalID, &gcpAccountCreatedAt, &gcpAccountSubscriptionStatus, &gcpAccountSubscriptionlevel,
	); err != nil {
		return nil, err
	}
	e.TeamExternalID = teamExternalID.String
	e.TeamName = teamName.String
	e.OrgID = orgID.String
	e.OrgExternalID = orgExternalID.String
	e.OrgName = orgName.String
	e.OrgCreatedAt = orgCreatedAt.Time
	e.Platform = platform.String
	e.Environment = environment.String
	e.ZuoraAccountNumber = zuoraAccountNumber.String
	e.GCPAccountExternalID = gcpAccountExternalID.String
	e.GCPAccountCreatedAt = gcpAccountCreatedAt.Time
	e.GCPAccountSubscriptionStatus = gcpAccountSubscriptionStatus.String
	e.GCPAccountSubscriptionLevel = gcpAccountSubscriptionlevel.String
	return e, nil
}
