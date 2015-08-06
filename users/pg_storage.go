package main

import (
	"database/sql"
	"time"

	"github.com/lib/pq"
	"github.com/weaveworks/service/users/names"
	"golang.org/x/crypto/bcrypt"
)

type pgStorage struct {
	*sql.DB
}

// Postgres only stores times to the microsecond, so we pre-truncate times so
// tests will match. We also normalize to UTC, for sanity.
func (s pgStorage) Now() time.Time {
	return time.Now().UTC().Truncate(time.Microsecond)
}

func (s pgStorage) CreateUser(email string) (*User, error) {
	u := &User{Email: email, CreatedAt: s.Now()}
	err := s.QueryRow("insert into users (email, created_at) values (lower($1), $2) returning id", email, u.CreatedAt).Scan(&u.ID)
	switch {
	case err == sql.ErrNoRows:
		return nil, ErrNotFound
	case err != nil:
		return nil, err
	}
	return u, nil
}

func (s pgStorage) FindUserByID(id string) (*User, error) {
	return s.findUserByID(s, id)
}

type QueryRower interface {
	QueryRow(query string, args ...interface{}) *sql.Row
}

func (s pgStorage) findUserByID(db QueryRower, id string) (*User, error) {
	user, err := s.scanUser(
		db.QueryRow(`
			select
					users.id, email, token, token_created_at, approved_at,
					users.created_at, organization_id, organizations.name,
					organizations.probe_token
				from users
				left join organizations on (users.organization_id = organizations.id)
				where users.id = $1
				and users.deleted_at is null`,
			id,
		),
	)
	if err == sql.ErrNoRows {
		err = ErrNotFound
	}
	return user, err
}

func (s pgStorage) FindUserByEmail(email string) (*User, error) {
	user, err := s.scanUser(
		s.QueryRow(`
			select
					users.id, email, token, token_created_at, approved_at,
					users.created_at, organization_id, organizations.name,
					organizations.probe_token
				from users
				left join organizations on (users.organization_id = organizations.id)
				where lower(email) = lower($1)
				and users.deleted_at is null`,
			email,
		),
	)
	if err == sql.ErrNoRows {
		err = ErrNotFound
	}
	return user, err
}

type scanner interface {
	Scan(...interface{}) error
}

// TODO: Scan more columns
func (s pgStorage) scanUser(row scanner) (*User, error) {
	u := &User{}
	var token, oID, oName, probeToken sql.NullString
	var createdAt, tokenCreatedAt, approvedAt pq.NullTime
	if err := row.Scan(&u.ID, &u.Email, &token, &tokenCreatedAt, &approvedAt, &createdAt, &oID, &oName, &probeToken); err != nil {
		return nil, err
	}
	s.setString(&u.Token, token)
	s.setString(&u.OrganizationID, oID)
	s.setString(&u.OrganizationName, oName)
	s.setString(&u.ProbeToken, probeToken)
	s.setTime(&u.TokenCreatedAt, tokenCreatedAt)
	s.setTime(&u.ApprovedAt, approvedAt)
	s.setTime(&u.CreatedAt, createdAt)
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

func (s pgStorage) ListUnapprovedUsers() ([]*User, error) {
	rows, err := s.Query(`
		select users.id, email, token, token_created_at, approved_at, users.created_at, organization_id, organizations.name
		from users
		left join organizations on (users.organization_id = organizations.id)
		where users.approved_at is null
		and users.deleted_at is null
		order by users.created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := []*User{}
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

func (s pgStorage) ApproveUser(id string) (*User, error) {
	var user *User
	err := s.Transaction(func(tx *sql.Tx) error {
		o, err := s.createOrganization(tx)
		if err != nil {
			return err
		}

		result, err := tx.Exec(`
			update users set
				organization_id = $2,
				approved_at = $3
			where id = $1 and deleted_at is null`,
			id,
			o.ID,
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
			return ErrNotFound
		}

		user, err = s.findUserByID(tx, id)
		return err
	})
	if err != nil {
		user = nil
	}
	return user, err
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
		return ErrNotFound
	}
	return nil
}

func (s pgStorage) createOrganization(db QueryRower) (*Organization, error) {
	probeToken, err := generateProbeToken()
	if err != nil {
		return nil, err
	}
	o := &Organization{
		Name:       names.Generate(),
		ProbeToken: probeToken,
		CreatedAt:  s.Now(),
	}
	err = db.QueryRow(`
		insert into organizations
			(name, probe_token, created_at)
			values ($1, $2, $3)
			returning id`,
		o.Name, o.ProbeToken, o.CreatedAt,
	).Scan(&o.ID)
	switch {
	case err == sql.ErrNoRows:
		return nil, ErrNotFound
	case err != nil:
		return nil, err
	}
	return o, nil
}

func (s pgStorage) FindOrganizationByName(name string) (*Organization, error) {
	o, err := s.scanOrganization(
		s.QueryRow(`
			select
					id, name, probe_token, first_probe_update_at, last_probe_update_at,
					created_at
				from organizations
				where lower(name) = lower($1)
				and deleted_at is null`,
			name,
		),
	)
	if err == sql.ErrNoRows {
		err = ErrNotFound
	}
	return o, err
}

func (s pgStorage) scanOrganization(row scanner) (*Organization, error) {
	o := &Organization{}
	var name, probeToken sql.NullString
	var firstProbeUpdatedAt, lastProbeUpdatedAt, createdAt pq.NullTime
	if err := row.Scan(&o.ID, &name, &probeToken, &firstProbeUpdatedAt, &lastProbeUpdatedAt, &createdAt); err != nil {
		return nil, err
	}
	s.setString(&o.Name, name)
	s.setString(&o.ProbeToken, probeToken)
	s.setTime(&o.FirstProbeUpdateAt, firstProbeUpdatedAt)
	s.setTime(&o.LastProbeUpdateAt, lastProbeUpdatedAt)
	s.setTime(&o.CreatedAt, createdAt)
	return o, nil
}

func (s pgStorage) Transaction(f func(*sql.Tx) error) error {
	tx, err := s.Begin()
	if err != nil {
		return err
	}
	if err = f(tx); err != nil {
		// Rollback error is ignored as we already have one in progress
		tx.Rollback()
		return err
	}
	return tx.Commit()
}
