package postgres

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/Masterminds/squirrel"
	"github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"

	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/db/filter"
	"github.com/weaveworks/service/users/login"
)

// CreateUser creates a new user with the given email.
func (d DB) CreateUser(_ context.Context, email string) (*users.User, error) {
	u := &users.User{Email: email, CreatedAt: d.Now()}
	err := d.QueryRow("insert into users (email, approved_at, created_at) values (lower($1), $2, $2) returning id", email, u.CreatedAt).Scan(&u.ID)
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
func (d DB) AddLoginToUser(ctx context.Context, userID, provider, providerID string, session json.RawMessage) error {
	now := d.Now()
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
	return d.Transaction(func(tx DB) error {
		// check that this is not already attached somewhere else
		existing, err := tx.FindUserByLogin(ctx, provider, providerID)
		switch err {
		case nil:
			if existing.ID != userID {
				return users.AlreadyAttachedError{ID: existing.ID, Email: existing.Email}
			}
			// User is already attached to this auth provider, just update the session
			_, err = tx.
				Update("logins").
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
			_, err = tx.
				Insert("logins").
				SetMap(values).
				Exec()
		}
		return err
	})
}

// DetachLoginFromUser detaches the specified login from a user. e.g. if you
// want to attach it to a different user, do this first.
func (d DB) DetachLoginFromUser(_ context.Context, userID, provider string) error {
	_, err := d.Exec(
		`update logins
			set deleted_at = $3
			where user_id = $1
			and provider = $2
			and deleted_at is null`,
		userID, provider, d.Now(),
	)
	return err
}

// InviteUser invites the user, to join the organization. If they are already a
// member this is a noop.
func (d DB) InviteUser(ctx context.Context, email, orgExternalID string) (*users.User, bool, error) {
	var u *users.User
	userCreated := false
	err := d.Transaction(func(tx DB) error {
		o, err := tx.scanOrganization(
			tx.organizationsQuery().Where("lower(organizations.external_id) = lower($1)", orgExternalID).QueryRow(),
		)
		if err != nil {
			return err
		}

		u, err = tx.FindUserByEmail(ctx, email)
		if err == users.ErrNotFound {
			u, err = tx.CreateUser(ctx, email)
			userCreated = true
		}
		if err != nil {
			return err
		}

		isMember, err := tx.UserIsMemberOf(ctx, u.ID, orgExternalID)
		if err != nil || isMember {
			return err
		}
		err = tx.addUserToOrganization(u.ID, o.ID)
		if err != nil {
			return err
		}
		u, err = tx.FindUserByID(ctx, u.ID)
		return err
	})
	if err != nil {
		return nil, false, err
	}
	return u, userCreated, nil
}

// FindUserByID finds the user by id
func (d DB) FindUserByID(_ context.Context, id string) (*users.User, error) {
	user, err := d.scanUser(
		d.usersQuery().Where(squirrel.Eq{"users.id": id}).QueryRow(),
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
func (d DB) FindUserByEmail(_ context.Context, email string) (*users.User, error) {
	user, err := d.scanUser(
		d.usersQuery().Where("lower(users.email) = lower($1)", email).QueryRow(),
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
func (d DB) FindUserByLogin(_ context.Context, provider, providerID string) (*users.User, error) {
	user, err := d.scanUser(
		d.usersQuery().
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

func (d DB) scanUsers(rows *sql.Rows) ([]*users.User, error) {
	users := []*users.User{}
	for rows.Next() {
		user, err := d.scanUser(rows)
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

func (d DB) scanUser(row squirrel.RowScanner) (*users.User, error) {
	u := &users.User{}
	var (
		token sql.NullString

		createdAt,
		firstLoginAt,
		tokenCreatedAt pq.NullTime
	)
	if err := row.Scan(
		&u.ID, &u.Email, &token, &tokenCreatedAt, &createdAt,
		&u.Admin, &firstLoginAt,
	); err != nil {
		return nil, err
	}
	u.Token = token.String
	u.TokenCreatedAt = tokenCreatedAt.Time
	u.CreatedAt = createdAt.Time
	u.FirstLoginAt = firstLoginAt.Time
	return u, nil
}

func (d DB) usersQuery() squirrel.SelectBuilder {
	return d.Select(
		"users.id",
		"users.email",
		"users.token",
		"users.token_created_at",
		"users.created_at",
		"users.admin",
		"users.first_login_at",
	).
		From("users").
		Where("users.deleted_at is null").
		OrderBy("users.created_at DESC")
}

// ListUsers lists users
func (d DB) ListUsers(_ context.Context, f filter.User, page uint64) ([]*users.User, error) {
	q := d.usersQuery().Where(f.Where())
	if page > 0 {
		q = q.Limit(filter.ResultsPerPage).Offset((page - 1) * filter.ResultsPerPage)
	}

	rows, err := q.Query()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return d.scanUsers(rows)
}

// ListLoginsForUserIDs lists the logins for these users
func (d DB) ListLoginsForUserIDs(_ context.Context, userIDs ...string) ([]*login.Login, error) {
	rows, err := d.Select(
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

// SetUserAdmin sets the admin flag of a user
func (d DB) SetUserAdmin(_ context.Context, id string, value bool) error {
	result, err := d.Exec(`
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
func (d DB) SetUserToken(_ context.Context, id, token string) error {
	var hashed []byte
	if token != "" {
		var err error
		hashed, err = bcrypt.GenerateFromPassword([]byte(token), d.PasswordHashingCost)
		if err != nil {
			return err
		}
	}
	result, err := d.Exec(`
		update users set
			token = $2,
			token_created_at = $3
		where id = $1 and deleted_at is null`,
		id,
		string(hashed),
		d.Now(),
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
func (d DB) SetUserFirstLoginAt(_ context.Context, id string) error {
	result, err := d.Exec(`
		update users set
			first_login_at = $2
		where id = $1
			and first_login_at is null
			and deleted_at is null`,
		id,
		d.Now(),
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
