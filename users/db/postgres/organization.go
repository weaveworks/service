package postgres

import (
	"database/sql"

	"github.com/Masterminds/squirrel"
	"github.com/lib/pq"

	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/externalIDs"
)

// RemoveUserFromOrganization removes the user from the organiation. If they
// are not a member, this is a noop.
func (s DB) RemoveUserFromOrganization(orgExternalID, email string) error {
	_, err := s.Exec(`
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
		s.Now(),
		email,
		orgExternalID,
	)
	return err
}

// UserIsMemberOf checks if the user is a member of the organization
func (s DB) UserIsMemberOf(userID, orgExternalID string) (bool, error) {
	return s.userIsMemberOf(s.DB, userID, orgExternalID)
}

func (s DB) userIsMemberOf(db squirrel.BaseRunner, userID, orgExternalID string) (bool, error) {
	rows, err := s.organizationsQuery().
		Join("memberships on (organizations.id = memberships.organization_id)").
		Where(squirrel.Eq{"memberships.user_id": userID, "organizations.external_id": orgExternalID}).
		Where("memberships.deleted_at is null").
		RunWith(db).
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

func (s DB) organizationsQuery() squirrel.SelectBuilder {
	return s.Select(
		"organizations.id",
		"organizations.external_id",
		"organizations.name",
		"organizations.probe_token",
		"organizations.first_probe_update_at",
		"organizations.created_at",
		"organizations.feature_flags",
	).
		From("organizations").
		Where("organizations.deleted_at is null").
		OrderBy("organizations.created_at")
}

// ListOrganizations lists organizations
func (s DB) ListOrganizations() ([]*users.Organization, error) {
	rows, err := s.organizationsQuery().Query()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return s.scanOrganizations(rows)
}

// ListOrganizationUsers lists all the users in an organization
func (s DB) ListOrganizationUsers(orgExternalID string) ([]*users.User, error) {
	rows, err := s.usersQuery().
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
	return s.scanUsers(rows)
}

// ListOrganizationsForUserIDs lists the organizations these users belong to
func (s DB) ListOrganizationsForUserIDs(userIDs ...string) ([]*users.Organization, error) {
	rows, err := s.organizationsQuery().
		Join("memberships on (organizations.id = memberships.organization_id)").
		Where(squirrel.Eq{"memberships.user_id": userIDs}).
		Where("memberships.deleted_at is null").
		Query()
	if err != nil {
		return nil, err
	}
	orgs, err := s.scanOrganizations(rows)
	if err != nil {
		return nil, err
	}
	return orgs, err
}

func (s DB) addUserToOrganization(db execQueryRower, userID, organizationID string) error {
	_, err := db.Exec(`
			insert into memberships
				(user_id, organization_id, created_at)
				values ($1, $2, $3)`,
		userID,
		organizationID,
		s.Now(),
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
func (s DB) GenerateOrganizationExternalID() (string, error) {
	var (
		externalID string
		err        error
	)
	err = s.Transaction(func(tx *sql.Tx) error {
		for exists := true; exists; {
			externalID = externalIDs.Generate()
			exists, err = s.organizationExists(tx, externalID)
		}
		return nil
	})
	return externalID, err
}

// CreateOrganization creates a new organization owned by the user
func (s DB) CreateOrganization(ownerID, externalID, name string) (*users.Organization, error) {
	o := &users.Organization{
		ExternalID: externalID,
		Name:       name,
		CreatedAt:  s.Now(),
	}
	if err := o.Valid(); err != nil {
		return nil, err
	}

	err := s.Transaction(func(tx *sql.Tx) error {
		if exists, err := s.organizationExists(tx, o.ExternalID); err != nil {
			return err
		} else if exists {
			return users.ErrOrgExternalIDIsTaken
		}

		for exists := o.ProbeToken == ""; exists; {
			if err := o.RegenerateProbeToken(); err != nil {
				return err
			}
			if err := tx.QueryRow(
				`select exists(select 1 from organizations where probe_token = $1 and deleted_at is null)`,
				o.ProbeToken,
			).Scan(&exists); err != nil {
				return err
			}
		}

		err := tx.QueryRow(`insert into organizations
			(external_id, name, probe_token, created_at)
			values (lower($1), $2, $3, $4) returning id`,
			o.ExternalID, o.Name, o.ProbeToken, o.CreatedAt,
		).Scan(&o.ID)
		if err != nil {
			return err
		}

		return s.addUserToOrganization(tx, ownerID, o.ID)
	})
	if err != nil {
		return nil, err
	}
	return o, err
}

// FindOrganizationByProbeToken looks up the organization matching a given
// probe token.
func (s DB) FindOrganizationByProbeToken(probeToken string) (*users.Organization, error) {
	var o *users.Organization
	var err error
	err = s.Transaction(func(tx *sql.Tx) error {
		o, err = s.scanOrganization(
			s.organizationsQuery().RunWith(tx).Where(squirrel.Eq{"organizations.probe_token": probeToken}).QueryRow(),
		)
		if err == nil && o.FirstProbeUpdateAt.IsZero() {
			o.FirstProbeUpdateAt = s.Now()
			_, err = tx.Exec(`update organizations set first_probe_update_at = $2 where id = $1`, o.ID, o.FirstProbeUpdateAt)
		}

		if err == sql.ErrNoRows {
			err = users.ErrNotFound
		}
		return err
	})
	if err != nil {
		o = nil
	}
	return o, err
}

func (s DB) scanOrganizations(rows *sql.Rows) ([]*users.Organization, error) {
	orgs := []*users.Organization{}
	for rows.Next() {
		org, err := s.scanOrganization(rows)
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

func (s DB) scanOrganization(row squirrel.RowScanner) (*users.Organization, error) {
	o := &users.Organization{}
	var externalID, name, probeToken sql.NullString
	var firstProbeUpdateAt, createdAt pq.NullTime
	if err := row.Scan(&o.ID, &externalID, &name, &probeToken, &firstProbeUpdateAt, &createdAt, pq.Array(&o.FeatureFlags)); err != nil {
		return nil, err
	}
	o.ExternalID = externalID.String
	o.Name = name.String
	o.ProbeToken = probeToken.String
	o.FirstProbeUpdateAt = firstProbeUpdateAt.Time
	o.CreatedAt = createdAt.Time
	return o, nil
}

// RenameOrganization changes an organization's user-settable name
func (s DB) RenameOrganization(externalID, name string) error {
	if err := (&users.Organization{ExternalID: externalID, Name: name}).Valid(); err != nil {
		return err
	}

	result, err := s.Exec(`
		update organizations set name = $2
		where lower(external_id) = lower($1) and deleted_at is null`,
		externalID, name,
	)
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
func (s DB) OrganizationExists(externalID string) (bool, error) {
	return s.organizationExists(s, externalID)
}

func (s DB) organizationExists(db queryRower, externalID string) (bool, error) {
	var exists bool
	err := db.QueryRow(
		`select exists(select 1 from organizations where lower(external_id) = lower($1) and deleted_at is null)`,
		externalID,
	).Scan(&exists)
	return exists, err
}

// GetOrganizationName gets the name of an organization from it's external ID.
func (s DB) GetOrganizationName(externalID string) (string, error) {
	var name string
	err := s.QueryRow(
		`select name from organizations where lower(external_id) = lower($1) and deleted_at is null`,
		externalID,
	).Scan(&name)
	return name, err
}

// DeleteOrganization deletes an organization
func (s DB) DeleteOrganization(externalID string) error {
	_, err := s.Exec(
		`update organizations set deleted_at = $1 where lower(external_id) = lower($2)`,
		s.Now(), externalID,
	)
	return err
}

// AddFeatureFlag adds a new feature flag to a organization.
func (s DB) AddFeatureFlag(externalID string, featureFlag string) error {
	_, err := s.Exec(
		`update organizations set feature_flags = feature_flags || $1 where lower(external_id) = lower($2)`,
		pq.Array([]string{featureFlag}), externalID,
	)
	return err
}
