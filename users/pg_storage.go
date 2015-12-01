package main

import (
	"database/sql"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/Sirupsen/logrus"
	"github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

type pgStorage struct {
	*sql.DB
	squirrel.StatementBuilderType
}

type queryRower interface {
	QueryRow(query string, args ...interface{}) *sql.Row
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

func (s pgStorage) InviteUser(email, orgName string) (*user, error) {
	err := s.Transaction(func(tx *sql.Tx) error {
		o, err := s.scanOrganization(
			tx.QueryRow(`
				select
						id, name, probe_token, first_probe_update_at, created_at
					from organizations
					where lower(name) = lower($1)
					and deleted_at is null`,
				orgName,
			),
		)
		if err != nil {
			return err
		}

		user, err := s.findUserByEmail(tx, email)
		if err == errNotFound {
			user, err = s.createUser(tx, email)
		}
		if err != nil {
			return err
		}

		if user.Organization != nil && user.Organization.Name != orgName {
			return errEmailIsTaken
		}

		return s.addUserToOrganization(tx, user.ID, o.ID)
	})
	if err != nil {
		return nil, err
	}
	return s.FindUserByEmail(email)
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
	user, err := s.scanUser(
		s.usersQuery().Where(squirrel.Eq{"users.id": id}).QueryRow(),
	)
	if err == sql.ErrNoRows {
		err = errNotFound
	}
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
		token,
		oID,
		oName,
		oProbeToken sql.NullString

		createdAt,
		firstLoginAt,
		tokenCreatedAt,
		approvedAt,
		oFirstProbeUpdateAt,
		oCreatedAt pq.NullTime
	)
	if err := row.Scan(
		&u.ID, &u.Email, &token, &tokenCreatedAt, &approvedAt, &createdAt,
		&firstLoginAt, &oID, &oName, &oProbeToken, &oFirstProbeUpdateAt,
		&oCreatedAt,
	); err != nil {
		return nil, err
	}
	s.setString(&u.Token, token)
	s.setTime(&u.TokenCreatedAt, tokenCreatedAt)
	s.setTime(&u.ApprovedAt, approvedAt)
	s.setTime(&u.CreatedAt, createdAt)
	s.setTime(&u.FirstLoginAt, firstLoginAt)
	if oID.Valid {
		o := &organization{}
		s.setString(&o.ID, oID)
		s.setString(&o.Name, oName)
		s.setString(&o.ProbeToken, oProbeToken)
		s.setTime(&o.FirstProbeUpdateAt, oFirstProbeUpdateAt)
		s.setTime(&o.CreatedAt, oCreatedAt)
		u.Organization = o
	}
	return u, nil
}

func (s pgStorage) setTime(dst *time.Time, src pq.NullTime) {
	if src.Valid {
		*dst = src.Time
	}
}

func (s pgStorage) setString(dst *string, src sql.NullString) {
	if src.Valid {
		*dst = src.String
	}
}

func (s pgStorage) usersQuery() squirrel.SelectBuilder {
	return s.Select(
		"users.id",
		"users.email",
		"users.token",
		"users.token_created_at",
		"users.approved_at",
		"users.created_at",
		"users.first_login_at",
		"users.organization_id",
		"organizations.name",
		"organizations.probe_token",
		"organizations.first_probe_update_at",
		"organizations.created_at",
	).
		From("users").
		LeftJoin("organizations on (users.organization_id = organizations.id)").
		Where("users.deleted_at is null").
		OrderBy("users.created_at")
}

func (s pgStorage) ListUsers(fs ...filter) ([]*user, error) {
	rows, err := s.applyFilters(s.usersQuery(), fs).Query()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return s.scanUsers(rows)
}

func (s pgStorage) ListOrganizationUsers(orgName string) ([]*user, error) {
	return s.ListUsers(newUsersOrganizationFilter([]string{orgName}))
}

func (s pgStorage) ApproveUser(id string) (*user, error) {
	err := s.Transaction(func(tx *sql.Tx) error {
		o, err := s.createOrganization(tx)
		if err != nil {
			return err
		}

		return s.addUserToOrganization(tx, id, o.ID)
	})
	if err != nil && err != errNotFound {
		return nil, err
	}
	return s.FindUserByID(id)
}

func (s pgStorage) addUserToOrganization(db squirrel.Execer, userID, organizationID string) error {
	_, err := db.Exec(`
			update users set
				organization_id = $2,
				approved_at = $3
			where id = $1
			and approved_at is null
			and deleted_at is null`,
		userID,
		organizationID,
		s.Now(),
	)
	return err
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

func (s pgStorage) createOrganization(db queryRower) (*organization, error) {
	var (
		o   = &organization{CreatedAt: s.Now()}
		err error
	)
	o.RegenerateName()
	if err := o.RegenerateProbeToken(); err != nil {
		return nil, err
	}

	for {
		err = db.QueryRow(`insert into organizations
			(name, probe_token, created_at)
			values ($1, $2, $3) returning id`,
			o.Name, o.ProbeToken, o.CreatedAt,
		).Scan(&o.ID)

		if e, ok := err.(*pq.Error); ok {
			switch e.Constraint {
			case "organizations_lower_name_idx":
				o.RegenerateName()
				continue
			case "organizations_probe_token_idx":
				if err := o.RegenerateProbeToken(); err != nil {
					return nil, err
				}
				continue
			}
		}
		break
	}

	if err != nil {
		o = nil
	}
	return o, err
}

func (s pgStorage) FindOrganizationByProbeToken(probeToken string) (*organization, error) {
	var o *organization
	var err error
	err = s.Transaction(func(tx *sql.Tx) error {
		o, err = s.scanOrganization(
			tx.QueryRow(`
				select
						id, name, probe_token, first_probe_update_at, created_at
					from organizations
					where probe_token = $1
					and deleted_at is null`,
				probeToken,
			),
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

func (s pgStorage) scanOrganization(row squirrel.RowScanner) (*organization, error) {
	o := &organization{}
	var name, probeToken sql.NullString
	var firstProbeUpdateAt, createdAt pq.NullTime
	if err := row.Scan(&o.ID, &name, &probeToken, &firstProbeUpdateAt, &createdAt); err != nil {
		return nil, err
	}
	s.setString(&o.Name, name)
	s.setString(&o.ProbeToken, probeToken)
	s.setTime(&o.FirstProbeUpdateAt, firstProbeUpdateAt)
	s.setTime(&o.CreatedAt, createdAt)
	return o, nil
}

func (s pgStorage) RenameOrganization(oldName, newName string) error {
	result, err := s.Exec(`
		update organizations set name = $2
		where lower(name) = lower($1) and deleted_at is null`,
		oldName, newName,
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
