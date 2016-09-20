package postgres

import (
	"database/sql"
	"encoding/json"

	"github.com/Masterminds/squirrel"
	"github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"

	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/login"
)

func (s pgDB) CreateUser(email string) (*users.User, error) {
	return s.createUser(s, email)
}

func (s pgDB) createUser(q queryRower, email string) (*users.User, error) {
	u := &users.User{Email: email, CreatedAt: s.Now()}
	err := q.QueryRow("insert into users (email, created_at) values (lower($1), $2) returning id", email, u.CreatedAt).Scan(&u.ID)
	switch {
	case err == sql.ErrNoRows:
		return nil, users.ErrNotFound
	case err != nil:
		return nil, err
	}
	return u, nil
}

func (s pgDB) AddLoginToUser(userID, provider, providerID string, session json.RawMessage) error {
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

func (s pgDB) DetachLoginFromUser(userID, provider string) error {
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

func (s pgDB) InviteUser(email, orgExternalID string) (*users.User, bool, error) {
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
		err = s.addUserToOrganization(tx, u.ID, o.ID)
		if err != nil {
			return err
		}
		u, err = s.findUserByID(tx, u.ID)
		return err
	})
	if err != nil {
		return nil, false, err
	}
	return u, userCreated, nil
}

func (s pgDB) FindUserByID(id string) (*users.User, error) {
	return s.findUserByID(s.DB, id)
}

func (s pgDB) findUserByID(db squirrel.BaseRunner, id string) (*users.User, error) {
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

func (s pgDB) FindUserByEmail(email string) (*users.User, error) {
	return s.findUserByEmail(s.DB, email)
}

func (s pgDB) findUserByEmail(db squirrel.BaseRunner, email string) (*users.User, error) {
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

func (s pgDB) FindUserByLogin(provider, providerID string) (*users.User, error) {
	return s.findUserByLogin(s.DB, provider, providerID)
}

func (s pgDB) findUserByLogin(db squirrel.BaseRunner, provider, providerID string) (*users.User, error) {
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

func (s pgDB) scanUsers(rows *sql.Rows) ([]*users.User, error) {
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

func (s pgDB) scanUser(row squirrel.RowScanner) (*users.User, error) {
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

func (s pgDB) usersQuery() squirrel.SelectBuilder {
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

func (s pgDB) ListUsers() ([]*users.User, error) {
	rows, err := s.usersQuery().Query()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return s.scanUsers(rows)
}

func (s pgDB) ListLoginsForUserIDs(userIDs ...string) ([]*login.Login, error) {
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

func (s pgDB) ApproveUser(id string) (*users.User, error) {
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

func (s pgDB) SetUserAdmin(id string, value bool) error {
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

func (s pgDB) SetUserToken(id, token string) error {
	var hashed []byte
	if token != "" {
		var err error
		hashed, err = bcrypt.GenerateFromPassword([]byte(token), s.PasswordHashingCost)
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

func (s pgDB) SetUserFirstLoginAt(id string) error {
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
