package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/lib/pq"

	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/db/filter"
	"github.com/weaveworks/service/users/externalIDs"
)

// RemoveUserFromOrganization removes the user from the organiation. If they
// are not a member, this is a noop.
func (d DB) RemoveUserFromOrganization(_ context.Context, orgExternalID, email string) error {
	_, err := d.Exec(`
			update memberships set deleted_at = $1
			where user_id in (
					select id
					  from users
					 where lower(email) = lower($2)
					   and deleted_at is null
				)
			  and organization_id in (
					select id
					  from organizations
					 where lower(external_id) = lower($3)
					   and deleted_at is null
				)
			  and deleted_at is null`,
		d.Now(),
		email,
		orgExternalID,
	)
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

func (d DB) userIsDirectMemberOf(_ context.Context, userID, orgExternalID string) (bool, error) {
	rows, err := d.organizationsQuery().
		Join("memberships on (organizations.id = memberships.organization_id)").
		Where(squirrel.Eq{"memberships.user_id": userID, "organizations.external_id": orgExternalID}).
		Where("memberships.deleted_at is null").
		Query()
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

func (d DB) userIsTeamMemberOf(_ context.Context, userID, orgExternalID string) (bool, error) {
	rows, err := d.organizationsQuery().
		Join("team_memberships on (organizations.team_id = team_memberships.team_id)").
		Where(squirrel.Eq{"team_memberships.user_id": userID, "organizations.external_id": orgExternalID}).
		Where("team_memberships.deleted_at IS NULL").
		Query()
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
	return d.Select(
		"organizations.id",
		"organizations.external_id",
		"organizations.name",
		"organizations.probe_token",
		"organizations.created_at",
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
	).
		From("organizations").
		LeftJoin("gcp_accounts ON gcp_account_id = gcp_accounts.id").
		Where("organizations.deleted_at is null").
		OrderBy("organizations.created_at DESC")
}

// ListOrganizations lists organizations
func (d DB) ListOrganizations(_ context.Context, f filter.Organization, page uint64) ([]*users.Organization, error) {
	q := d.organizationsQuery().Where(f.Where())
	if page > 0 {
		q = q.Limit(filter.ResultsPerPage).Offset((page - 1) * filter.ResultsPerPage)
	}

	rows, err := q.Query()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return d.scanOrganizations(rows)
}

// ListOrganizationUsers lists all the users in an organization
func (d DB) ListOrganizationUsers(_ context.Context, orgExternalID string) ([]*users.User, error) {
	rows, err := d.usersQuery().
		Join("memberships on (memberships.user_id = users.id)").
		Join("organizations on (memberships.organization_id = organizations.id)").
		Where(squirrel.Eq{
			"organizations.external_id": orgExternalID,
			"memberships.deleted_at":    nil,
			"organizations.deleted_at":  nil,
		}).
		OrderBy("users.created_at").
		Query()
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
	return mergeOrgs(memberOrgs, teamOrgs), nil
}

// listMemberOrganizationsForUserIDs lists the organizations these users belong to
func (d DB) listMemberOrganizationsForUserIDs(_ context.Context, userIDs ...string) ([]*users.Organization, error) {
	rows, err := d.organizationsQuery().
		Join("memberships on (organizations.id = memberships.organization_id)").
		Where(squirrel.Eq{"memberships.user_id": userIDs}).
		Where("memberships.deleted_at is null").
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

// listTeamOrganizationsForUserIDs lists the organizations these users' teams belong to
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

func (d DB) addUserToOrganization(userID, organizationID string) error {
	_, err := d.Exec(`
			insert into memberships
				(user_id, organization_id, created_at)
				values ($1, $2, $3)`,
		userID,
		organizationID,
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
			externalID = externalIDs.Generate()
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
func (d DB) CreateOrganization(ctx context.Context, ownerID, externalID, name, token string) (*users.Organization, error) {
	now := d.Now()
	o := &users.Organization{
		ExternalID:     externalID,
		Name:           name,
		CreatedAt:      now,
		TrialExpiresAt: now.Add(users.TrialDuration),
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

			if err := tx.QueryRow(
				`select exists(select 1 from organizations where probe_token = $1 and deleted_at is null)`,
				o.ProbeToken,
			).Scan(&exists); err != nil {
				return err
			}
			if token != "" && exists {
				return users.ErrOrgTokenIsTaken
			}
		}

		o.TeamID, err = tx.ensureTeamExists(ctx, ownerID)
		if err != nil {
			return err
		}

		err = tx.QueryRow(`insert into organizations
			(external_id, name, probe_token, created_at, trial_expires_at, team_id)
			values (lower($1), $2, $3, $4, $5, $6) returning id`,
			o.ExternalID, o.Name, o.ProbeToken, o.CreatedAt, o.TrialExpiresAt, o.TeamID,
		).Scan(&o.ID)

		return err
	})
	if err != nil {
		return nil, err
	}
	return o, err
}

func (d DB) ensureTeamExists(ctx context.Context, ownerID string) (string, error) {
	// Which team does the organization go into?
	// * if the user has a default team, pick that
	// * if a user is not part of a team, create the team and the organization within that team
	var teamID string
	err := d.Transaction(func(tx DB) error {
		team, err := tx.DefaultTeamByUserID(ownerID)
		if err != nil && err != sql.ErrNoRows {
			return err
		}
		if team != nil {
			teamID = team.ID
			return nil
		}

		// user has no team
		team, err = tx.CreateTeam(ctx, ownerID)
		if err != nil {
			return err
		}
		err = tx.addUserToTeam(ownerID, team.ID)
		if err != nil {
			return err
		}
		err = tx.SetDefaultTeam(ownerID, team.ID)
		if err != nil {
			return err
		}
		teamID = team.ID
		return nil
	})
	return teamID, err
}

// FindOrganizationByProbeToken looks up the organization matching a given
// probe token.
func (d DB) FindOrganizationByProbeToken(_ context.Context, probeToken string) (*users.Organization, error) {
	o, err := d.scanOrganization(
		d.organizationsQuery().Where(squirrel.Eq{"organizations.probe_token": probeToken}).QueryRow(),
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
func (d DB) FindOrganizationByID(_ context.Context, externalID string) (*users.Organization, error) {
	o, err := d.scanOrganization(
		d.organizationsQuery().Where(squirrel.Eq{"organizations.external_id": externalID}).QueryRow(),
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
		d.organizationsQuery().Where(squirrel.Eq{"organizations.gcp_account_id": gcp.ID}).QueryRow(),
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
		d.organizationsQuery().Where(squirrel.Eq{"organizations.id": internalID}).QueryRow(),
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
	var externalID, name, probeToken, platform, environment, zuoraAccountNumber, teamID sql.NullString
	var createdAt pq.NullTime
	var firstSeenConnectedAt, zuoraAccountCreatedAt *time.Time
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
		pq.Array(&o.FeatureFlags),
		&refuseDataAccess,
		&refuseDataUpload,
		&firstSeenConnectedAt,
		&platform,
		&environment,
		&zuoraAccountNumber,
		&zuoraAccountCreatedAt,
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
	); err != nil {
		return nil, err
	}
	o.ExternalID = externalID.String
	o.Name = name.String
	o.ProbeToken = probeToken.String
	o.CreatedAt = createdAt.Time
	o.RefuseDataAccess = refuseDataAccess
	o.RefuseDataUpload = refuseDataUpload
	o.FirstSeenConnectedAt = firstSeenConnectedAt
	o.Platform = platform.String
	o.Environment = environment.String
	o.ZuoraAccountNumber = zuoraAccountNumber.String
	o.ZuoraAccountCreatedAt = zuoraAccountCreatedAt
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
	return o, nil
}

// UpdateOrganization changes an organization's user-settable name
func (d DB) UpdateOrganization(ctx context.Context, externalID string, update users.OrgWriteView) error {
	// Get org for validation and add update fields to setFields
	org, err := d.FindOrganizationByID(ctx, externalID)
	if err != nil {
		return err
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
		org.TrialPendingExpiryNotifiedAt = update.TrialPendingExpiryNotifiedAt
		setFields["trial_pending_expiry_notified_at"] = *update.TrialPendingExpiryNotifiedAt
	}
	if update.TrialExpiredNotifiedAt != nil {
		org.TrialExpiredNotifiedAt = update.TrialExpiredNotifiedAt
		setFields["trial_expired_notified_at"] = *update.TrialExpiredNotifiedAt
	}

	if len(setFields) == 0 {
		return nil
	}

	if err := org.Valid(); err != nil {
		return err
	}

	result, err := d.Update("organizations").
		SetMap(setFields).
		Where(squirrel.Expr("external_id = lower(?) and deleted_at is null", externalID)).
		Exec()
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	switch {
	case err != nil:
		return err
	case count != 1:
		return users.ErrNotFound
	}
	return nil
}

// OrganizationExists just returns a simple bool checking if an organization
// exists. It exists if it hasn't been deleted.
func (d DB) OrganizationExists(_ context.Context, externalID string) (bool, error) {
	var exists bool
	err := d.QueryRow(
		`select exists(select 1 from organizations where external_id = lower($1) and deleted_at is null)`,
		externalID,
	).Scan(&exists)
	return exists, err
}

// ExternalIDUsed returns true if the given `externalID` has ever been in use for
// an organization.
func (d DB) ExternalIDUsed(_ context.Context, externalID string) (bool, error) {
	var exists bool
	err := d.QueryRow(
		`select exists(select 1 from organizations where external_id = lower($1))`,
		externalID,
	).Scan(&exists)
	return exists, err
}

// GetOrganizationName gets the name of an organization from its external ID.
func (d DB) GetOrganizationName(_ context.Context, externalID string) (string, error) {
	var name string
	err := d.QueryRow(
		`select name from organizations where external_id = lower($1) and deleted_at is null`,
		externalID,
	).Scan(&name)
	return name, err
}

// DeleteOrganization deletes an organization
func (d DB) DeleteOrganization(_ context.Context, externalID string) error {
	_, err := d.Exec(
		`update organizations set deleted_at = $1 where external_id = lower($2) and deleted_at is null`,
		d.Now(), externalID,
	)
	return err
}

// AddFeatureFlag adds a new feature flag to a organization.
func (d DB) AddFeatureFlag(_ context.Context, externalID string, featureFlag string) error {
	_, err := d.Exec(
		`update organizations set feature_flags = feature_flags || $1 where external_id = lower($2) and deleted_at is null`,
		pq.Array([]string{featureFlag}), externalID,
	)
	return err
}

// SetFeatureFlags sets all feature flags of an organization.
func (d DB) SetFeatureFlags(_ context.Context, externalID string, featureFlags []string) error {
	if featureFlags == nil {
		featureFlags = []string{}
	}
	_, err := d.Exec(
		`update organizations set feature_flags = $1 where external_id = lower($2) and deleted_at is null`,
		pq.Array(featureFlags), externalID,
	)
	return err
}

// SetOrganizationRefuseDataAccess sets the "deny UI features" flag on an organization
func (d DB) SetOrganizationRefuseDataAccess(_ context.Context, externalID string, value bool) error {
	_, err := d.Exec(
		`update organizations set refuse_data_access = $1 where external_id = lower($2) and deleted_at is null`,
		value, externalID,
	)
	return err
}

// SetOrganizationRefuseDataUpload sets the "deny token auth" flag on an organization
func (d DB) SetOrganizationRefuseDataUpload(_ context.Context, externalID string, value bool) error {
	_, err := d.Exec(
		`update organizations set refuse_data_upload = $1 where external_id = lower($2) and deleted_at is null`,
		value, externalID,
	)
	return err
}

// SetOrganizationFirstSeenConnectedAt sets the first time an organisation has been connected
func (d DB) SetOrganizationFirstSeenConnectedAt(_ context.Context, externalID string, value *time.Time) error {
	_, err := d.Exec(
		`update organizations set first_seen_connected_at = $1 where external_id = lower($2) and deleted_at is null`,
		value, externalID,
	)
	return err
}

// SetOrganizationZuoraAccount sets the account number and time it was created at.
func (d DB) SetOrganizationZuoraAccount(_ context.Context, externalID, number string, createdAt *time.Time) error {
	_, err := d.Exec(
		`update organizations set zuora_account_number = $1, zuora_account_created_at = $2 where external_id = lower($3) and deleted_at is null`,
		number, createdAt, externalID,
	)
	return err
}

// CreateOrganizationWithGCP creates an organization with an inactive GCP account attached to it.
func (d DB) CreateOrganizationWithGCP(ctx context.Context, ownerID, externalAccountID string) (*users.Organization, error) {
	var org *users.Organization
	var gcp *users.GoogleCloudPlatform
	err := d.Transaction(func(tx DB) error {
		externalID, err := tx.GenerateOrganizationExternalID(ctx)
		if err != nil {
			return err
		}
		name := users.DefaultOrganizationName(externalID)
		org, err = tx.CreateOrganization(ctx, ownerID, externalID, name, "")
		if err != nil {
			return err
		}

		// Create and attach inactive GCP subscription to the organization
		gcp, err = tx.createGCP(ctx, externalAccountID)
		if err != nil {
			return err
		}

		err = tx.SetOrganizationGCP(ctx, externalID, externalAccountID)
		if err != nil {
			return err
		}
		return nil
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
	err := d.QueryRow(
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
	_, err := d.Exec(
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

		_, err = d.Exec(
			`update organizations set gcp_account_id = $1 where external_id = $2 and deleted_at is null`,
			gcp.ID, externalID,
		)
		if err != nil {
			return err
		}

		platform, env := "kubernetes", "gke"
		now := d.Now()
		if err = tx.UpdateOrganization(ctx, externalID, users.OrgWriteView{
			// Hardcode platform/env here, that's what we expect the user to have.
			// It also skips the platform/env tab during the onboarding process.
			Platform:    &platform,
			Environment: &env,
			// No trial for GCP instances
			TrialExpiresAt: &now,
		}); err != nil {
			return err
		}

		return tx.AddFeatureFlag(ctx, externalID, users.BillingFeatureFlag)
	})
}

// createGCP creates a Google Cloud Platform account/subscription. It is initialized as inactive.
func (d DB) createGCP(ctx context.Context, externalAccountID string) (*users.GoogleCloudPlatform, error) {
	now := d.Now()
	gcp := &users.GoogleCloudPlatform{
		ExternalAccountID: externalAccountID,
		CreatedAt:         now,
	}
	err := d.QueryRow(`insert into gcp_accounts
			(external_account_id, created_at, activated)
			values ($1, $2, false) returning id`,
		gcp.ExternalAccountID, gcp.CreatedAt).
		Scan(&gcp.ID)
	if err != nil {
		return nil, err
	}

	return gcp, nil
{

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
