package postgres

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/lib/pq"
	"golang.org/x/net/context"

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

// UserIsMemberOf checks if the user is a member of the organization
func (d DB) UserIsMemberOf(_ context.Context, userID, orgExternalID string) (bool, error) {
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
	return ok, rows.Close()
}

func (d DB) organizationsQuery() squirrel.SelectBuilder {
	return d.Select(
		"organizations.id",
		"organizations.external_id",
		"organizations.name",
		"organizations.probe_token",
		"organizations.created_at",
		"organizations.feature_flags",
		"organizations.deny_ui_features",
		"organizations.deny_token_auth",
		"organizations.first_seen_connected_at",
		"organizations.platform",
		"organizations.environment",
		"organizations.zuora_account_number",
		"organizations.zuora_account_created_at",
	).
		From("organizations").
		Where("organizations.deleted_at is null").
		OrderBy("organizations.created_at DESC")
}

// ListOrganizations lists organizations
func (d DB) ListOrganizations(_ context.Context, f filter.Organization) ([]*users.Organization, error) {
	q := d.organizationsQuery()
	q = f.ExtendQuery(q)

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
		Query()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return d.scanUsers(rows)
}

// ListOrganizationsForUserIDs lists the organizations these users belong to
func (d DB) ListOrganizationsForUserIDs(_ context.Context, userIDs ...string) ([]*users.Organization, error) {
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
		for exists := true; exists; {
			externalID = externalIDs.Generate()
			exists, terr = tx.OrganizationExists(ctx, externalID)
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
	o := &users.Organization{
		ExternalID: externalID,
		Name:       name,
		CreatedAt:  d.Now(),
	}
	if err := o.Valid(); err != nil {
		return nil, err
	}

	err := d.Transaction(func(tx DB) error {
		exists, err := tx.OrganizationExists(ctx, o.ExternalID)
		if err != nil {
			return err
		}
		if exists {
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

		err = tx.QueryRow(`insert into organizations
			(external_id, name, probe_token, created_at)
			values (lower($1), $2, $3, $4) returning id`,
			o.ExternalID, o.Name, o.ProbeToken, o.CreatedAt,
		).Scan(&o.ID)
		if err != nil {
			return err
		}

		return tx.addUserToOrganization(ownerID, o.ID)
	})
	if err != nil {
		return nil, err
	}
	trialExpiry, err := users.CalculateTrialExpiry(o.CreatedAt, []string{})
	if err != nil {
		panic(fmt.Sprintf("Could not calculate trial expiry: %v", err))
	}
	o.TrialExpiresAt = trialExpiry
	return o, err
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
	var externalID, name, probeToken, platform, environment, zuoraAccountNumber sql.NullString
	var createdAt pq.NullTime
	var firstSeenConnectedAt, zuoraAccountCreatedAt *time.Time
	var denyUIFeatures, denyTokenAuth bool
	if err := row.Scan(
		&o.ID,
		&externalID,
		&name,
		&probeToken,
		&createdAt,
		pq.Array(&o.FeatureFlags),
		&denyUIFeatures,
		&denyTokenAuth,
		&firstSeenConnectedAt,
		&platform,
		&environment,
		&zuoraAccountNumber,
		&zuoraAccountCreatedAt,
	); err != nil {
		return nil, err
	}
	o.ExternalID = externalID.String
	o.Name = name.String
	o.ProbeToken = probeToken.String
	o.CreatedAt = createdAt.Time
	o.DenyUIFeatures = denyUIFeatures
	o.DenyTokenAuth = denyTokenAuth
	o.FirstSeenConnectedAt = firstSeenConnectedAt
	o.Platform = platform.String
	o.Environment = environment.String
	o.ZuoraAccountNumber = zuoraAccountNumber.String
	o.ZuoraAccountCreatedAt = zuoraAccountCreatedAt

	// TODO: Store trial expiry in the database, rather than deriving from these fields
	trialExpiry, err := users.CalculateTrialExpiry(o.CreatedAt, o.FeatureFlags)
	if err != nil {
		return nil, err
	}
	o.TrialExpiresAt = trialExpiry
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

	if len(setFields) == 0 {
		return nil
	}

	if err := org.Valid(); err != nil {
		return err
	}

	result, err := d.Update("organizations").
		SetMap(setFields).
		Where(squirrel.Expr("lower(external_id) = lower(?) and deleted_at is null", externalID)).
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
// exists
func (d DB) OrganizationExists(_ context.Context, externalID string) (bool, error) {
	var exists bool
	err := d.QueryRow(
		`select exists(select 1 from organizations where lower(external_id) = lower($1) and deleted_at is null)`,
		externalID,
	).Scan(&exists)
	return exists, err
}

// GetOrganizationName gets the name of an organization from it's external ID.
func (d DB) GetOrganizationName(_ context.Context, externalID string) (string, error) {
	var name string
	err := d.QueryRow(
		`select name from organizations where lower(external_id) = lower($1) and deleted_at is null`,
		externalID,
	).Scan(&name)
	return name, err
}

// DeleteOrganization deletes an organization
func (d DB) DeleteOrganization(_ context.Context, externalID string) error {
	_, err := d.Exec(
		`update organizations set deleted_at = $1 where lower(external_id) = lower($2)`,
		d.Now(), externalID,
	)
	return err
}

// AddFeatureFlag adds a new feature flag to a organization.
func (d DB) AddFeatureFlag(_ context.Context, externalID string, featureFlag string) error {
	_, err := d.Exec(
		`update organizations set feature_flags = feature_flags || $1 where lower(external_id) = lower($2) and deleted_at is null`,
		pq.Array([]string{featureFlag}), externalID,
	)
	return err
}

// SetFeatureFlags sets all feature flags of an organization.
func (d DB) SetFeatureFlags(_ context.Context, externalID string, featureFlags []string) error {
	if featureFlags == nil {
		featureFlags = make([]string, 0)
	}
	_, err := d.Exec(
		`update organizations set feature_flags = $1 where lower(external_id) = lower($2) and deleted_at is null`,
		pq.Array(featureFlags), externalID,
	)
	return err
}

// SetOrganizationDenyUIFeatures sets the "deny UI features" flag on an organization
func (d DB) SetOrganizationDenyUIFeatures(_ context.Context, externalID string, value bool) error {
	_, err := d.Exec(
		`update organizations set deny_ui_features = $1 where lower(external_id) = lower($2) and deleted_at is null`,
		value, externalID,
	)
	return err
}

// SetOrganizationDenyTokenAuth sets the "deny token auth" flag on an organization
func (d DB) SetOrganizationDenyTokenAuth(_ context.Context, externalID string, value bool) error {
	_, err := d.Exec(
		`update organizations set deny_token_auth = $1 where lower(external_id) = lower($2) and deleted_at is null`,
		value, externalID,
	)
	return err
}

// SetOrganizationFirstSeenConnectedAt sets the first time an organisation has been connected
func (d DB) SetOrganizationFirstSeenConnectedAt(_ context.Context, externalID string, value *time.Time) error {
	_, err := d.Exec(
		`update organizations set first_seen_connected_at = $1 where lower(external_id) = lower($2) and deleted_at is null`,
		value, externalID,
	)
	return err
}

// SetOrganizationZuoraAccount sets the account number and time it was created at.
func (d DB) SetOrganizationZuoraAccount(_ context.Context, externalID, number string, createdAt *time.Time) error {
	_, err := d.Exec(
		`update organizations set zuora_account_number = $1, zuora_account_created_at = $2 where lower(external_id) = lower($3) and deleted_at is null`,
		number, createdAt, externalID,
	)
	return err
}
