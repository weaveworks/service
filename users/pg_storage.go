package main

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/Sirupsen/logrus"
	"github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"

	"github.com/weaveworks/service/users/login"
	"github.com/weaveworks/service/users/names"
)

type pgStorage struct {
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

func newPGStorage(db *sql.DB) *pgStorage {
	return &pgStorage{
		DB:                   db,
		StatementBuilderType: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar).RunWith(db),
	}
}

// Postgres only stores times to the microsecond, so we pre-truncate times so
// tests will match. We also normalize to UTC, for sanity.
func (s pgStorage) Now() time.Time {
	return time.Now().UTC().Truncate(time.Microsecond)
}

func (s pgStorage) CreateUser(email string) (*user, error) {
	return s.createUser(s, email)
}

func (s pgStorage) createUser(db queryRower, email string) (*user, error) {
	u := &user{Email: email, CreatedAt: s.Now()}
	err := db.QueryRow("insert into users (email, created_at) values (lower($1), $2) returning id", email, u.CreatedAt).Scan(&u.ID)
	switch {
	case err == sql.ErrNoRows:
		return nil, errNotFound
	case err != nil:
		return nil, err
	}
	return u, nil
}

func (s pgStorage) AddLoginToUser(userID, provider, providerID string, session json.RawMessage) error {
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
		// remove any attachments to other users this auth provider/account might
		// have.
		_, err := tx.Exec(
			`update logins
				set deleted_at = $1
				where provider = $2
				and provider_id = $3
				and user_id != $4
				and deleted_at is null`,
			now, provider, providerID, userID,
		)
		if err != nil {
			return err
		}
		var id string
		err = tx.QueryRow(
			`select id from logins
				where provider = $1
				and provider_id = $2
				and user_id = $3
				and deleted_at is null`,
			provider, providerID, userID,
		).Scan(&id)
		switch err {
		case nil:
			// User is already attached to this auth provider, just update the session
			_, err = s.
				Update("logins").
				RunWith(tx).
				Where(squirrel.Eq{"id": id}).
				SetMap(values).
				Exec()
		case sql.ErrNoRows:
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

func (s pgStorage) DetachLoginFromUser(userID, provider string) error {
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

func (s pgStorage) InviteUser(email, orgName string) (*user, error) {
	var u *user
	err := s.Transaction(func(tx *sql.Tx) error {
		o, err := s.scanOrganization(
			s.organizationsQuery().RunWith(tx).Where("lower(organizations.name) = lower($1)", orgName).QueryRow(),
		)
		if err != nil {
			return err
		}

		u, err = s.findUserByEmail(tx, email)
		if err == errNotFound {
			u, err = s.createUser(tx, email)
		}
		if err != nil {
			return err
		}

		switch len(u.Organizations) {
		case 0:
			if err = s.addUserToOrganization(tx, u.ID, o.ID); err == nil {
				u, err = s.findUserByID(tx, u.ID)
				return err
			}
		case 1:
			if u.Organizations[0].Name == orgName {
				return nil
			}
		}
		return errEmailIsTaken
	})
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (s pgStorage) DeleteUser(email string) error {
	_, err := s.Exec(`
			update users set deleted_at = $2
			where lower(email) = lower($1) and deleted_at is null`,
		email,
		s.Now(),
	)
	return err
}

func (s pgStorage) FindUserByID(id string) (*user, error) {
	return s.findUserByID(s.DB, id)
}

func (s pgStorage) findUserByID(db squirrel.BaseRunner, id string) (*user, error) {
	user, err := s.scanUser(
		s.usersQuery().RunWith(db).Where(squirrel.Eq{"users.id": id}).QueryRow(),
	)
	if err == sql.ErrNoRows {
		err = errNotFound
	}
	if err != nil {
		return nil, err
	}
	user.Organizations, err = s.listOrganizationsForUserIDs(db, id)
	if err != nil {
		return nil, err
	}
	user.Logins, err = s.listLoginsForUserIDs(db, id)
	return user, err
}

func (s pgStorage) FindUserByEmail(email string) (*user, error) {
	return s.findUserByEmail(s.DB, email)
}

func (s pgStorage) findUserByEmail(db squirrel.BaseRunner, email string) (*user, error) {
	user, err := s.scanUser(
		s.usersQuery().RunWith(db).Where("lower(users.email) = lower($1)", email).QueryRow(),
	)
	if err == sql.ErrNoRows {
		err = errNotFound
	}
	if err != nil {
		return nil, err
	}
	user.Organizations, err = s.listOrganizationsForUserIDs(db, user.ID)
	if err != nil {
		return nil, err
	}
	user.Logins, err = s.listLoginsForUserIDs(db, user.ID)
	return user, err
}

func (s pgStorage) FindUserByLogin(provider, providerID string) (*user, error) {
	user, err := s.scanUser(
		s.usersQuery().
			Join("logins on (logins.user_id = users.id)").
			Where(squirrel.Eq{
				"logins.provider":    provider,
				"logins.provider_id": providerID,
			}).
			Where("logins.deleted_at is null"),
	)
	if err == sql.ErrNoRows {
		err = errNotFound
	}
	if err != nil {
		return nil, err
	}
	user.Organizations, err = s.listOrganizationsForUserIDs(s.DB, user.ID)
	if err != nil {
		return nil, err
	}
	user.Logins, err = s.listLoginsForUserIDs(s.DB, user.ID)
	return user, err
}

func (s pgStorage) scanUsers(rows *sql.Rows) ([]*user, error) {
	users := []*user{}
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

func (s pgStorage) scanUser(row squirrel.RowScanner) (*user, error) {
	u := &user{}
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

func (s pgStorage) usersQuery() squirrel.SelectBuilder {
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

func (s pgStorage) organizationsQuery() squirrel.SelectBuilder {
	return s.Select(
		"organizations.id",
		"organizations.name",
		"organizations.label",
		"organizations.probe_token",
		"organizations.first_probe_update_at",
		"organizations.created_at",
	).
		From("organizations").
		Where("organizations.deleted_at is null").
		OrderBy("organizations.created_at")
}

func (s pgStorage) ListUsers(fs ...filter) ([]*user, error) {
	rows, err := s.applyFilters(s.usersQuery(), fs).Query()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users, err := s.scanUsers(rows)
	if err != nil {
		return nil, err
	}
	if len(users) == 0 {
		return users, nil
	}

	for _, user := range users {
		user.Organizations, err = s.listOrganizationsForUserIDs(s.DB, user.ID)
		if err != nil {
			return nil, err
		}
	}

	userIDs := []string{}
	usersByID := map[string]*user{}
	for _, user := range users {
		userIDs = append(userIDs, user.ID)
		usersByID[user.ID] = user
	}
	logins, err := s.listLoginsForUserIDs(s.DB, userIDs...)
	if err != nil {
		return nil, err
	}
	for _, a := range logins {
		if user, ok := usersByID[a.UserID]; ok {
			user.Logins = append(user.Logins, a)
		}
	}

	return users, err
}

func (s pgStorage) ListOrganizationUsers(orgName string) ([]*user, error) {
	return s.ListUsers(newUsersOrganizationFilter([]string{orgName}))
}

func (s pgStorage) listOrganizationsForUserIDs(db squirrel.BaseRunner, userIDs ...string) ([]*organization, error) {
	rows, err := s.organizationsQuery().
		RunWith(db).
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

func (s pgStorage) listLoginsForUserIDs(db squirrel.BaseRunner, userIDs ...string) ([]*login.Login, error) {
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
		RunWith(db).
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

func (s pgStorage) ApproveUser(id string) (*user, error) {
	var user *user
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

func (s pgStorage) addUserToOrganization(db execQueryRower, userID, organizationID string) error {
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

// Set the admin flag of a user
func (s pgStorage) SetUserAdmin(id string, value bool) error {
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
		return errNotFound
	}
	return nil
}

func (s pgStorage) SetUserToken(id, token string) error {
	var hashed []byte
	if token != "" {
		var err error
		hashed, err = bcrypt.GenerateFromPassword([]byte(token), passwordHashingCost)
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
		return errNotFound
	}
	return nil
}

func (s pgStorage) SetUserFirstLoginAt(id string) error {
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
		return errNotFound
	}
	return nil
}

func (s pgStorage) GenerateOrganizationName() (string, error) {
	var (
		name string
		err  error
	)
	err = s.Transaction(func(tx *sql.Tx) error {
		for exists := true; exists; {
			name = names.Generate()
			exists, err = s.organizationExists(tx, name)
		}
		return nil
	})
	return name, err
}

func (s pgStorage) CreateOrganization(ownerID, name, label string) (*organization, error) {
	o := &organization{
		Name:      name,
		Label:     label,
		CreatedAt: s.Now(),
	}
	if err := o.Valid(); err != nil {
		return nil, err
	}

	err := s.Transaction(func(tx *sql.Tx) error {
		if exists, err := s.organizationExists(tx, o.Name); err != nil {
			return err
		} else if exists {
			return errOrgNameIsTaken
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
			(name, label, probe_token, created_at)
			values ($1, $2, $3, $4) returning id`,
			o.Name, o.Label, o.ProbeToken, o.CreatedAt,
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

func (s pgStorage) FindOrganizationByProbeToken(probeToken string) (*organization, error) {
	var o *organization
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
			err = errNotFound
		}
		return err
	})
	if err != nil {
		o = nil
	}
	return o, err
}

func (s pgStorage) scanOrganizations(rows *sql.Rows) ([]*organization, error) {
	orgs := []*organization{}
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

func (s pgStorage) scanOrganization(row squirrel.RowScanner) (*organization, error) {
	o := &organization{}
	var name, label, probeToken sql.NullString
	var firstProbeUpdateAt, createdAt pq.NullTime
	if err := row.Scan(&o.ID, &name, &label, &probeToken, &firstProbeUpdateAt, &createdAt); err != nil {
		return nil, err
	}
	o.Name = name.String
	o.Label = label.String
	o.ProbeToken = probeToken.String
	o.FirstProbeUpdateAt = firstProbeUpdateAt.Time
	o.CreatedAt = createdAt.Time
	return o, nil
}

func (s pgStorage) RelabelOrganization(name, label string) error {
	if err := (&organization{Name: name, Label: label}).Valid(); err != nil {
		return err
	}

	result, err := s.Exec(`
		update organizations set label = $2
		where lower(name) = lower($1) and deleted_at is null`,
		name, label,
	)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	switch {
	case err != nil:
		return err
	case count != 1:
		return errNotFound
	}
	return nil
}

func (s pgStorage) OrganizationExists(name string) (bool, error) {
	return s.organizationExists(s, name)
}

func (s pgStorage) organizationExists(db queryRower, name string) (bool, error) {
	var exists bool
	err := db.QueryRow(
		`select exists(select 1 from organizations where lower(name) = lower($1) and deleted_at is null)`,
		name,
	).Scan(&exists)
	return exists, err
}

func (s pgStorage) Transaction(f func(*sql.Tx) error) error {
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

func (s pgStorage) applyFilters(q squirrel.SelectBuilder, fs []filter) squirrel.SelectBuilder {
	for _, f := range fs {
		q = f.Select(q)
	}
	return q
}
