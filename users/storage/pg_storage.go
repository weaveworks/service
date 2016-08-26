package storage

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/Sirupsen/logrus"
	"github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"

	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/externalIDs"
	"github.com/weaveworks/service/users/login"
)

// PGStorage is exposed for testing
type PGStorage struct {
	*sql.DB
	squirrel.StatementBuilderType
}

type queryRower interface {
	QueryRow(query string, args ...interface{}) *sql.Row
}

type execer interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
}

type execQueryRower interface {
	execer
	queryRower
}

func newPGStorage(db *sql.DB) *PGStorage {
	return &PGStorage{
		DB:                   db,
		StatementBuilderType: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar).RunWith(db),
	}
}

// Now gives us the current time for Postgres. Postgres only stores times to
// the microsecond, so we pre-truncate times so tests will match. We also
// normalize to UTC, for sanity.
func (s PGStorage) Now() time.Time {
	return time.Now().UTC().Truncate(time.Microsecond)
}

// CreateUser creates a new user with the given email.
func (s PGStorage) CreateUser(email string) (*users.User, error) {
	return s.createUser(s, email)
}

func (s PGStorage) createUser(db queryRower, email string) (*users.User, error) {
	u := &users.User{Email: email, CreatedAt: s.Now()}
	err := db.QueryRow("insert into users (email, created_at) values (lower($1), $2) returning id", email, u.CreatedAt).Scan(&u.ID)
	switch {
	case err == sql.ErrNoRows:
		return nil, users.ErrNotFound
	case err != nil:
		return nil, err
	}
	return u, nil
}

// AddLoginToUser adds the given login to the specified user. If it is already
// attached elsewhere, this will error.
func (s PGStorage) AddLoginToUser(userID, provider, providerID string, session json.RawMessage) error {
	now := s.Now()
	values := map[string]interface{}{
		"user_id":     userID,
		"provider":    provider,
		"provider_id": providerID,
	}
	if len(session) > 0 {
		sessionJSON, err := session.MarshalJSON()
		if err != nil {
			return err
		}
		values["session"] = sessionJSON
	}
	return s.Transaction(func(tx *sql.Tx) error {
		// check that this is not already attached somewhere else
		existing, err := s.findUserByLogin(tx, provider, providerID)
		switch err {
		case nil:
			if existing.ID != userID {
				return users.AlreadyAttachedError{ID: existing.ID, Email: existing.Email}
			}
			// User is already attached to this auth provider, just update the session
			_, err = s.
				Update("logins").
				RunWith(tx).
				Where(squirrel.Eq{
					"user_id":     userID,
					"provider":    provider,
					"provider_id": providerID,
				}).
				SetMap(values).
				Exec()
		case users.ErrNotFound:
			err = nil
			// User is not attached to this auth provider, attach them
			values["created_at"] = now
			_, err = s.
				Insert("logins").
				RunWith(tx).
				SetMap(values).
				Exec()
		}
		return err
	})
}

// DetachLoginFromUser detaches the specified login from a user. e.g. if you
// want to attach it to a different user, do this first.
func (s PGStorage) DetachLoginFromUser(userID, provider string) error {
	_, err := s.Exec(
		`update logins
			set deleted_at = $3
			where user_id = $1
			and provider = $2
			and deleted_at is null`,
		userID, provider, s.Now(),
	)
	return err
}

// CreateAPIToken creates an api token for the user
func (s PGStorage) CreateAPIToken(userID, description string) (*users.APIToken, error) {
	t := &users.APIToken{
		UserID:      userID,
		Description: description,
		CreatedAt:   s.Now(),
	}

	err := s.Transaction(func(tx *sql.Tx) error {
		for exists := t.Token == ""; exists; {
			if err := t.RegenerateToken(); err != nil {
				return err
			}
			if err := tx.QueryRow(
				`select exists(select 1 from api_tokens where token = $1 and deleted_at is null)`,
				t.Token,
			).Scan(&exists); err != nil {
				return err
			}
		}

		return tx.QueryRow(`insert into api_tokens
			(user_id, token, description, created_at)
			values (lower($1), $2, $3, $4) returning id`,
			t.UserID, t.Token, t.Description, t.CreatedAt,
		).Scan(&t.ID)
	})
	if err != nil {
		return nil, err
	}
	return t, err
}

// DeleteAPIToken deletes an api token for the user
func (s PGStorage) DeleteAPIToken(userID, token string) error {
	_, err := s.Exec(
		`update api_tokens
			set deleted_at = $3
			where user_id = $1
			and token = $2
			and deleted_at is null`,
		userID, token, s.Now(),
	)
	return err
}

// InviteUser invites the user, to join the organization. If they are already a
// member this is a noop.
func (s PGStorage) InviteUser(email, orgExternalID string) (*users.User, bool, error) {
	var u *users.User
	userCreated := false
	err := s.Transaction(func(tx *sql.Tx) error {
		o, err := s.scanOrganization(
			s.organizationsQuery().RunWith(tx).Where("lower(organizations.external_id) = lower($1)", orgExternalID).QueryRow(),
		)
		if err != nil {
			return err
		}

		u, err = s.findUserByEmail(tx, email)
		if err == users.ErrNotFound {
			u, err = s.createUser(tx, email)
			userCreated = true
		}
		if err != nil {
			return err
		}

		isMember, err := s.userIsMemberOf(tx, u.ID, orgExternalID)
		if err != nil || isMember {
			return err
		}
		if err = s.addUserToOrganization(tx, u.ID, o.ID); err == nil {
			u, err = s.findUserByID(tx, u.ID)
			return err
		}
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return u, userCreated, nil
}

// RemoveUserFromOrganization removes the user from the organiation. If they
// are not a member, this is a noop.
func (s PGStorage) RemoveUserFromOrganization(orgExternalID, email string) error {
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

// FindUserByID finds the user by id
func (s PGStorage) FindUserByID(id string) (*users.User, error) {
	return s.findUserByID(s.DB, id)
}

func (s PGStorage) findUserByID(db squirrel.BaseRunner, id string) (*users.User, error) {
	user, err := s.scanUser(
		s.usersQuery().RunWith(db).Where(squirrel.Eq{"users.id": id}).QueryRow(),
	)
	if err == sql.ErrNoRows {
		err = users.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}

// FindUserByEmail finds the user by email
func (s PGStorage) FindUserByEmail(email string) (*users.User, error) {
	return s.findUserByEmail(s.DB, email)
}

func (s PGStorage) findUserByEmail(db squirrel.BaseRunner, email string) (*users.User, error) {
	user, err := s.scanUser(
		s.usersQuery().RunWith(db).Where("lower(users.email) = lower($1)", email).QueryRow(),
	)
	if err == sql.ErrNoRows {
		err = users.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}

// FindUserByLogin finds the user by login
func (s PGStorage) FindUserByLogin(provider, providerID string) (*users.User, error) {
	return s.findUserByLogin(s.DB, provider, providerID)
}

func (s PGStorage) findUserByLogin(db squirrel.BaseRunner, provider, providerID string) (*users.User, error) {
	user, err := s.scanUser(
		s.usersQuery().
			RunWith(db).
			Join("logins on (logins.user_id = users.id)").
			Where(squirrel.Eq{
				"logins.provider":    provider,
				"logins.provider_id": providerID,
			}).
			Where("logins.deleted_at is null").
			QueryRow(),
	)
	if err == sql.ErrNoRows {
		err = users.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}

// FindUserByAPIToken finds a user by their api token
func (s PGStorage) FindUserByAPIToken(token string) (*users.User, error) {
	user, err := s.scanUser(
		s.usersQuery().
			Join("api_tokens on (api_tokens.user_id = users.id)").
			Where(squirrel.Eq{"api_tokens.token": token}).
			Where("api_tokens.deleted_at is null"),
	)
	if err == sql.ErrNoRows {
		err = users.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}

// UserIsMemberOf checks if the user is a member of the organization
func (s PGStorage) UserIsMemberOf(userID, orgExternalID string) (bool, error) {
	return s.userIsMemberOf(s.DB, userID, orgExternalID)
}

func (s PGStorage) userIsMemberOf(db squirrel.BaseRunner, userID, orgExternalID string) (bool, error) {
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

func (s PGStorage) scanUsers(rows *sql.Rows) ([]*users.User, error) {
	users := []*users.User{}
	for rows.Next() {
		user, err := s.scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return users, nil
}

func (s PGStorage) scanUser(row squirrel.RowScanner) (*users.User, error) {
	u := &users.User{}
	var (
		token sql.NullString

		createdAt,
		firstLoginAt,
		tokenCreatedAt,
		approvedAt pq.NullTime
	)
	if err := row.Scan(
		&u.ID, &u.Email, &token, &tokenCreatedAt, &approvedAt, &createdAt,
		&u.Admin, &firstLoginAt,
	); err != nil {
		return nil, err
	}
	u.Token = token.String
	u.TokenCreatedAt = tokenCreatedAt.Time
	u.ApprovedAt = approvedAt.Time
	u.CreatedAt = createdAt.Time
	u.FirstLoginAt = firstLoginAt.Time
	return u, nil
}

func (s PGStorage) usersQuery() squirrel.SelectBuilder {
	return s.Select(
		"users.id",
		"users.email",
		"users.token",
		"users.token_created_at",
		"users.approved_at",
		"users.created_at",
		"users.admin",
		"users.first_login_at",
	).
		From("users").
		Where("users.deleted_at is null").
		OrderBy("users.created_at")
}

func (s PGStorage) organizationsQuery() squirrel.SelectBuilder {
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

// ListUsers lists users, with some filters.
func (s PGStorage) ListUsers() ([]*users.User, error) {
	rows, err := s.usersQuery().Query()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return s.scanUsers(rows)
}

// ListOrganizationUsers lists all the users in an organization
func (s PGStorage) ListOrganizationUsers(orgExternalID string) ([]*users.User, error) {
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
func (s PGStorage) ListOrganizationsForUserIDs(userIDs ...string) ([]*users.Organization, error) {
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

// ListLoginsForUserIDs lists the logins for these users
func (s PGStorage) ListLoginsForUserIDs(userIDs ...string) ([]*login.Login, error) {
	rows, err := s.Select(
		"logins.user_id",
		"logins.provider",
		"logins.provider_id",
		"logins.session",
		"logins.created_at",
	).
		From("logins").
		Where(squirrel.Eq{"logins.user_id": userIDs}).
		Where("logins.deleted_at is null").
		OrderBy("logins.provider").
		Query()
	if err != nil {
		return nil, err
	}
	ls := []*login.Login{}
	for rows.Next() {
		l := &login.Login{}
		var userID, provider, providerID sql.NullString
		var session []byte
		var createdAt pq.NullTime
		if err := rows.Scan(&userID, &provider, &providerID, &session, &createdAt); err != nil {
			return nil, err
		}
		l.UserID = userID.String
		l.Provider = provider.String
		l.ProviderID = providerID.String
		l.CreatedAt = createdAt.Time
		l.Session = session
		ls = append(ls, l)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return ls, err
}

// ListAPITokensForUserIDs lists the api tokens for these users
func (s PGStorage) ListAPITokensForUserIDs(userIDs ...string) ([]*users.APIToken, error) {
	rows, err := s.Select(
		"api_tokens.id",
		"api_tokens.user_id",
		"api_tokens.token",
		"api_tokens.description",
		"api_tokens.created_at",
	).
		From("api_tokens").
		Where(squirrel.Eq{"api_tokens.user_id": userIDs}).
		Where("api_tokens.deleted_at is null").
		OrderBy("api_tokens.id asc").
		Query()
	if err != nil {
		return nil, err
	}
	ts := []*users.APIToken{}
	for rows.Next() {
		t := &users.APIToken{}
		var userID, token, description sql.NullString
		var createdAt pq.NullTime
		if err := rows.Scan(&t.ID, &userID, &token, &description, &createdAt); err != nil {
			return nil, err
		}
		t.UserID = userID.String
		t.Token = token.String
		t.Description = description.String
		t.CreatedAt = createdAt.Time
		ts = append(ts, t)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return ts, err
}

// ApproveUser approves a user. Sort of deprecated, as all users are
// auto-approved now.
func (s PGStorage) ApproveUser(id string) (*users.User, error) {
	var user *users.User
	err := s.Transaction(func(tx *sql.Tx) error {
		result, err := tx.Exec(`update users set approved_at = $2 where id = $1 and approved_at is null`, id, s.Now())
		if err != nil {
			return err
		}

		if _, err := result.RowsAffected(); err != nil {
			return err
		}

		user, err = s.findUserByID(tx, id)
		return err
	})
	return user, err
}

func (s PGStorage) addUserToOrganization(db execQueryRower, userID, organizationID string) error {
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

// SetUserAdmin sets the admin flag of a user
func (s PGStorage) SetUserAdmin(id string, value bool) error {
	result, err := s.Exec(`
		update users set admin = $2 where id = $1 and deleted_at is null
	`, id, value,
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

// SetUserToken updates the user's login token
func (s PGStorage) SetUserToken(id, token string) error {
	var hashed []byte
	if token != "" {
		var err error
		hashed, err = bcrypt.GenerateFromPassword([]byte(token), PasswordHashingCost)
		if err != nil {
			return err
		}
	}
	result, err := s.Exec(`
		update users set
			token = $2,
			token_created_at = $3
		where id = $1 and deleted_at is null`,
		id,
		string(hashed),
		s.Now(),
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

// SetUserFirstLoginAt is called the first time a user logs in, to set their
// first_login_at field.
func (s PGStorage) SetUserFirstLoginAt(id string) error {
	result, err := s.Exec(`
		update users set
			first_login_at = $2
		where id = $1
			and first_login_at is null
			and deleted_at is null`,
		id,
		s.Now(),
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

// GenerateOrganizationExternalID returns an available organization external
// id, e.g. creaky-door-97
// TODO: There is a known issue, where as we fill up the database this will
// gradually slow down (since the algorithm is quite naive). We should fix it
// eventually.
func (s PGStorage) GenerateOrganizationExternalID() (string, error) {
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
func (s PGStorage) CreateOrganization(ownerID, externalID, name string) (*users.Organization, error) {
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
func (s PGStorage) FindOrganizationByProbeToken(probeToken string) (*users.Organization, error) {
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

func (s PGStorage) scanOrganizations(rows *sql.Rows) ([]*users.Organization, error) {
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

func (s PGStorage) scanOrganization(row squirrel.RowScanner) (*users.Organization, error) {
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
func (s PGStorage) RenameOrganization(externalID, name string) error {
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
func (s PGStorage) OrganizationExists(externalID string) (bool, error) {
	return s.organizationExists(s, externalID)
}

func (s PGStorage) organizationExists(db queryRower, externalID string) (bool, error) {
	var exists bool
	err := db.QueryRow(
		`select exists(select 1 from organizations where lower(external_id) = lower($1) and deleted_at is null)`,
		externalID,
	).Scan(&exists)
	return exists, err
}

// GetOrganizationName gets the name of an organization from it's external ID.
func (s PGStorage) GetOrganizationName(externalID string) (string, error) {
	var name string
	err := s.QueryRow(
		`select name from organizations where lower(external_id) = lower($1) and deleted_at is null`,
		externalID,
	).Scan(&name)
	return name, err
}

// DeleteOrganization deletes an organization
func (s PGStorage) DeleteOrganization(externalID string) error {
	_, err := s.Exec(
		`update organizations set deleted_at = $1 where lower(external_id) = lower($2)`,
		s.Now(), externalID,
	)
	return err
}

// AddFeatureFlag adds a new feature flag to a user.
func (s PGStorage) AddFeatureFlag(externalID string, featureFlag string) error {
	_, err := s.Exec(
		`update organizations set feature_flags = feature_flags || $1 where lower(external_id) = lower($2)`,
		pq.Array([]string{featureFlag}), externalID,
	)
	return err
}

// Transaction runs the given function in a postgres transaction. If fn returns
// an error the txn will be rolled back.
func (s PGStorage) Transaction(f func(*sql.Tx) error) error {
	tx, err := s.Begin()
	if err != nil {
		return err
	}
	if err = f(tx); err != nil {
		// Rollback error is ignored as we already have one in progress
		if err2 := tx.Rollback(); err2 != nil {
			logrus.Warn("transaction rollback: %v (ignored)", err2)
		}
		return err
	}
	return tx.Commit()
}

// Truncate clears all the data in pg. Should only be used in tests!
func (s PGStorage) Truncate() error {
	return mustExec(
		s,
		`truncate table traceable;`,
		`truncate table users;`,
		`truncate table logins;`,
		`truncate table organizations;`,
		`truncate table memberships;`,
	)
}

func mustExec(db squirrel.Execer, queries ...string) error {
	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			return err
		}
	}
	return nil
}
