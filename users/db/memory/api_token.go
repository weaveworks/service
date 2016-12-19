package memory

import (
	"fmt"
	"time"

	"github.com/weaveworks/service/users"
)

// CreateAPIToken creates an api token for the user
func (d *DB) CreateAPIToken(userID, description string) (*users.APIToken, error) {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	if _, err := d.findUserByID(userID); err != nil {
		return nil, err
	}
	t := &users.APIToken{
		ID:          fmt.Sprint(len(d.apiTokens)),
		UserID:      userID,
		Description: description,
		CreatedAt:   time.Now().UTC(),
	}
	for exists := t.Token == ""; exists; {
		if err := t.RegenerateToken(); err != nil {
			return nil, err
		}
		_, exists = d.apiTokens[t.Token]
	}
	d.apiTokens[t.Token] = t
	return t, nil
}

// DeleteAPIToken deletes an api token for the user
func (d *DB) DeleteAPIToken(userID, token string) error {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	if _, err := d.findUserByID(userID); err != nil {
		return err
	}
	existing, ok := d.apiTokens[token]
	if !ok || existing.UserID != userID {
		return nil
	}
	delete(d.apiTokens, token)
	return nil
}

// FindUserByAPIToken finds a user by their api token
func (d *DB) FindUserByAPIToken(token string) (*users.User, error) {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	t, ok := d.apiTokens[token]
	if !ok {
		return nil, users.ErrNotFound
	}
	return d.findUserByID(t.UserID)
}

// ListAPITokensForUserIDs lists the api tokens for these users
func (d *DB) ListAPITokensForUserIDs(userIDs ...string) ([]*users.APIToken, error) {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	var tokens []*users.APIToken
	for _, t := range d.apiTokens {
		for _, userID := range userIDs {
			if t.UserID == userID {
				tokens = append(tokens, t)
				break
			}
		}
	}
	return tokens, nil
}
