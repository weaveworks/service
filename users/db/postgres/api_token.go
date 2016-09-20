package postgres

import (
	"database/sql"

	"github.com/Masterminds/squirrel"
	"github.com/lib/pq"

	"github.com/weaveworks/service/users"
)

// CreateAPIToken creates an api token for the user
func (s DB) CreateAPIToken(userID, description string) (*users.APIToken, error) {
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
func (s DB) DeleteAPIToken(userID, token string) error {
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

// FindUserByAPIToken finds a user by their api token
func (s DB) FindUserByAPIToken(token string) (*users.User, error) {
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

// ListAPITokensForUserIDs lists the api tokens for these users
func (s DB) ListAPITokensForUserIDs(userIDs ...string) ([]*users.APIToken, error) {
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
