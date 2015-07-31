package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	mathRand "math/rand"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/dustinkirkland/golang-petname"
	"github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrNotFound = errors.New("Not found")
)

type Storage interface {
	CreateUser(email string) (*User, error)
	FindUserByID(id string) (*User, error)
	FindUserByEmail(email string) (*User, error)

	ListUnapprovedUsers() ([]*User, error)
	// Approve the user for access. Should generate them a new organization.
	ApproveUser(id string) error

	// Update the user's login token. Setting the token to "" should disable the
	// user's token.
	SetUserToken(id, token string) error

	Close() error
}

func setupStorage(databaseURI string) {
	db, err := sql.Open("postgres", databaseURI)
	if err != nil {
		logrus.Fatal(err)
	}
	storage = &sqlStorage{db}
}

type sqlStorage struct {
	*sql.DB
}

// Postgres only stores times to the microsecond, so we pre-truncate times so
// tests will match. We also normalize to UTC, for sanity.
func pgNow() time.Time {
	return time.Now().UTC().Truncate(time.Microsecond)
}

func (s sqlStorage) CreateUser(email string) (*User, error) {
	u := &User{Email: email, CreatedAt: pgNow()}
	err := s.QueryRow("insert into users (email, created_at) values (lower($1), $2) returning id", email, u.CreatedAt).Scan(&u.ID)
	switch {
	case err == sql.ErrNoRows:
		return nil, ErrNotFound
	case err != nil:
		return nil, err
	}
	return u, nil
}

func (s sqlStorage) FindUserByID(id string) (*User, error) {
	return s.findUserByID(s, id)
}

type QueryRower interface {
	QueryRow(query string, args ...interface{}) *sql.Row
}

func (s sqlStorage) findUserByID(db QueryRower, id string) (*User, error) {
	user, err := scanUser(
		db.QueryRow(`
			select users.id, email, token, token_created_at, approved_at, users.created_at, organization_id, organizations.name
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

func (s sqlStorage) FindUserByEmail(email string) (*User, error) {
	user, err := scanUser(
		s.QueryRow(`
			select users.id, email, token, token_created_at, approved_at, users.created_at, organization_id, organizations.name
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
func scanUser(row scanner) (*User, error) {
	u := &User{}
	var token, oID, oName sql.NullString
	var createdAt, tokenCreatedAt, approvedAt pq.NullTime
	if err := row.Scan(&u.ID, &u.Email, &token, &tokenCreatedAt, &approvedAt, &createdAt, &oID, &oName); err != nil {
		return nil, err
	}
	setString(&u.Token, token)
	setString(&u.OrganizationID, oID)
	setString(&u.OrganizationName, oName)
	setTime(&u.TokenCreatedAt, tokenCreatedAt)
	setTime(&u.ApprovedAt, approvedAt)
	setTime(&u.CreatedAt, createdAt)
	return u, nil
}

func setTime(dst *time.Time, src pq.NullTime) {
	if src.Valid {
		*dst = src.Time
	}
}

func setString(dst *string, src sql.NullString) {
	if src.Valid {
		*dst = src.String
	}
}

func (s sqlStorage) ListUnapprovedUsers() ([]*User, error) {
	rows, err := s.Query(`
		select users.id, email, token, token_created_at, approved_at, users.created_at, organization_id, organizations.name
		from users
		left join organizations on (users.organization_id = organizations.id)
		where users.approved_at is null
		and users.deleted_at is null`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := []*User{}
	for rows.Next() {
		user, err := scanUser(rows)
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

func (s sqlStorage) ApproveUser(id string) error {
	return s.Transaction(func(tx *sql.Tx) error {
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
			pgNow(),
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
	})
}

func (s sqlStorage) SetUserToken(id, token string) error {
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
		pgNow(),
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

func secureRandomBase64(byteCount int) (string, error) {
	randomData := make([]byte, byteCount)
	_, err := rand.Read(randomData)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(randomData), nil
}

func (s sqlStorage) createOrganization(db QueryRower) (*Organization, error) {
	o := &Organization{
		Name: fmt.Sprintf("%s-%d", petname.Generate(2, "-"), mathRand.Int31n(100)),
	}
	err := db.QueryRow("insert into organizations (name) values ($1) returning id", o.Name).Scan(&o.ID)
	switch {
	case err == sql.ErrNoRows:
		return nil, ErrNotFound
	case err != nil:
		return nil, err
	}
	return o, nil
}

func (s sqlStorage) Transaction(f func(*sql.Tx) error) error {
	tx, err := s.Begin()
	if err != nil {
		return err
	}
	if err = f(tx); err != nil {
		// Rollback error is ignored as we already have one in progress
		tx.Rollback()
	}
	return tx.Commit()
}
