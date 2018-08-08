package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"

	"github.com/Masterminds/squirrel"
	"github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"

	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/db/filter"
	"github.com/weaveworks/service/users/login"
)

// CreateUser creates a new user with the given email.
func (d DB) CreateUser(ctx context.Context, email string) (*users.User, error) {
	u := &users.User{Email: email, Company: "", Name: "", CreatedAt: d.Now()}
	err := d.QueryRowContext(ctx, "insert into users (email, approved_at, created_at, company, name) values (lower($1), $2, $2, $3, $4) returning id", email, u.CreatedAt, u.Company, u.Name).Scan(&u.ID)
	switch {
	case err == sql.ErrNoRows:
		return nil, users.ErrNotFound
	case err != nil:
		return nil, err
	}
	return u, nil
}

// UpdateUser applies a UserUpdate to a *users.User
func (d DB) UpdateUser(ctx context.Context, userID string, update *users.UserUpdate) (*users.User, error) {
	var user *users.User
	values := map[string]interface{}{}

	user, err := d.FindUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	if update.Email != "" {
		email := strings.TrimSpace(update.Email)
		user.Email = email
		values["email"] = email
	}

	if update.Company != "" {
		company := strings.TrimSpace(update.Company)
		user.Company = company
		values["company"] = company
	}

	if update.Name != "" {
		name := strings.TrimSpace(update.Name)
		user.Name = name
		values["name"] = name
	}

	err = d.Transaction(func(tx DB) error {
		_, err := tx.
			Update("users").
			Where(squirrel.Eq{
				"id": userID,
			}).
			SetMap(values).
			Exec()

		return err
	})

	if err != nil {
		return nil, err
	}

	return user, nil
}

// DeleteUser marks a user as deleted. It also removes the user from memberships and triggers a deletion of organizations
// where the user was the lone member.
func (d DB) DeleteUser(ctx context.Context, userID string) error {
	deleted := d.Now()
	return d.Transaction(func(tx DB) error {
		// All organizations with this user as sole member
		rows, err := d.QueryContext(ctx, `
select o.external_id from organizations o join memberships m on m.organization_id=o.id
where exists(select 1 from memberships mi where mi.user_id=$1 and o.id=mi.organization_id and mi.deleted_at is null)
      and o.deleted_at is null
      and m.deleted_at is null
group by o.id
having count(m.id)=1;
`, userID)
		if err != nil {
			return err
		}

		var ids []string
		var externalID string
		for rows.Next() {
			if err := rows.Scan(&externalID); err != nil {
				return err
			}
			ids = append(ids, externalID)
		}

		for _, externalID := range ids {
			if err := d.DeleteOrganization(ctx, externalID); err != nil {
				return err
			}
		}

		// Delete organization memberships
		if _, err = d.ExecContext(ctx,
			`update memberships
			set deleted_at=$1
			where user_id=$2
			and deleted_at is null`,
			deleted, userID); err != nil {
			return err
		}

		// Delete team memberships
		if _, err = d.ExecContext(ctx,
			`update team_memberships
			set deleted_at=$1
			where user_id=$2
			and deleted_at is null`,
			deleted, userID); err != nil {
			return err
		}

		// Delete user
		if _, err = d.ExecContext(ctx,
			`update users
			set deleted_at = $1
			where id = $2
			and deleted_at is null`,
			deleted, userID); err != nil {
			return err
		}

		return nil
	})
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
				return &users.AlreadyAttachedError{ID: existing.ID, Email: existing.Email}
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
func (d DB) DetachLoginFromUser(ctx context.Context, userID, provider string) error {
	_, err := d.ExecContext(ctx,
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

		if o.TeamID == "" {
			isMember, err := tx.UserIsMemberOf(ctx, u.ID, orgExternalID)
			if err != nil || isMember {
				return err
			}
			err = tx.addUserToOrganization(ctx, u.ID, o.ID)
			if err != nil {
				return err
			}
		} else {
			err := tx.AddUserToTeam(ctx, u.ID, o.TeamID)
			if err != nil {
				return nil
			}
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
func (d DB) FindUserByID(ctx context.Context, id string) (*users.User, error) {
	user, err := d.scanUser(
		d.usersQuery().Where(squirrel.Eq{"users.id": id}).QueryRowContext(ctx),
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
func (d DB) FindUserByEmail(ctx context.Context, email string) (*users.User, error) {
	user, err := d.scanUser(
		d.usersQuery().Where("lower(users.email) = lower($1)", email).QueryRowContext(ctx),
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
func (d DB) FindUserByLogin(ctx context.Context, provider, providerID string) (*users.User, error) {
	user, err := d.scanUser(
		d.usersQuery().
			Join("logins on (logins.user_id = users.id)").
			Where(squirrel.Eq{
				"logins.provider":    provider,
				"logins.provider_id": providerID,
			}).
			Where("logins.deleted_at is null").
			QueryRowContext(ctx),
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
		lastLoginAt,
		tokenCreatedAt pq.NullTime
	)
	if err := row.Scan(
		&u.ID, &u.Email, &u.Company, &u.Name, &token, &tokenCreatedAt, &createdAt,
		&u.Admin, &firstLoginAt, &lastLoginAt,
	); err != nil {
		return nil, err
	}
	u.Token = token.String
	u.TokenCreatedAt = tokenCreatedAt.Time
	u.CreatedAt = createdAt.Time
	u.FirstLoginAt = firstLoginAt.Time
	u.LastLoginAt = lastLoginAt.Time
	return u, nil
}

func (d DB) usersQuery() squirrel.SelectBuilder {
	return d.Select(
		"users.id",
		"users.email",
		"users.company",
		"users.name",
		"users.token",
		"users.token_created_at",
		"users.created_at",
		"users.admin",
		"users.first_login_at",
		"users.last_login_at",
	).
		From("users").
		Where("users.deleted_at is null").
		OrderBy("users.created_at DESC")
}

// ListUsers lists users
func (d DB) ListUsers(ctx context.Context, f filter.User, page uint64) ([]*users.User, error) {
	q := d.usersQuery().Where(f.Where())
	if page > 0 {
		q = q.Limit(filter.ResultsPerPage).Offset((page - 1) * filter.ResultsPerPage)
	}

	rows, err := q.QueryContext(ctx)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return d.scanUsers(rows)
}

// ListLoginsForUserIDs lists the logins for these users
func (d DB) ListLoginsForUserIDs(ctx context.Context, userIDs ...string) ([]*login.Login, error) {
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
func (d DB) SetUserAdmin(ctx context.Context, id string, value bool) error {
	result, err := d.ExecContext(ctx, `
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
func (d DB) SetUserToken(ctx context.Context, id, token string) error {
	var hashed []byte
	if token != "" {
		var err error
		hashed, err = bcrypt.GenerateFromPassword([]byte(token), d.PasswordHashingCost)
		if err != nil {
			return err
		}
	}
	result, err := d.ExecContext(ctx, `
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

// SetUserLastLoginAt is called the ever ytime a user logs in, to set their
// lasst_login_at field.
// If it also is their forst login, first_login_at is also set
func (d DB) SetUserLastLoginAt(ctx context.Context, id string) error {
	now := d.Now()
	return d.Transaction(func(tx DB) error {
		var firstLoginAt pq.NullTime
		err := tx.QueryRowContext(ctx, `
			update users set
				last_login_at = $2
			where id = $1
				and deleted_at is null
			returning
				first_login_at`,
			id,
			now,
		).Scan(&firstLoginAt)
		if err != nil {
			return err
		}
		// not null: saves a query
		if firstLoginAt.Valid {
			return nil
		}
		result, err := tx.ExecContext(ctx, `
			update users set
				first_login_at = $2
			where id = $1
				and first_login_at is null
				and deleted_at is null`,
			id,
			now,
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
	})
}

type usersByCreatedAt []*users.User

func (u usersByCreatedAt) Len() int           { return len(u) }
func (u usersByCreatedAt) Swap(i, j int)      { u[i], u[j] = u[j], u[i] }
func (u usersByCreatedAt) Less(i, j int) bool { return u[i].CreatedAt.After(u[j].CreatedAt) }
