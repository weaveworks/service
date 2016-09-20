package memory

import (
	"fmt"
	"time"

	"github.com/weaveworks/service/users"
)

// CreateAPIToken creates an api token for the user
func (s *DB) CreateAPIToken(userID, description string) (*users.APIToken, error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	if _, err := s.findUserByID(userID); err != nil {
		return nil, err
	}
	t := &users.APIToken{
		ID:          fmt.Sprint(len(s.apiTokens)),
		UserID:      userID,
		Description: description,
		CreatedAt:   time.Now().UTC(),
	}
	for exists := t.Token == ""; exists; {
		if err := t.RegenerateToken(); err != nil {
			return nil, err
		}
		_, exists = s.apiTokens[t.Token]
	}
	s.apiTokens[t.Token] = t
	return t, nil
}

// DeleteAPIToken deletes an api token for the user
func (s *DB) DeleteAPIToken(userID, token string) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	if _, err := s.findUserByID(userID); err != nil {
		return err
	}
	existing, ok := s.apiTokens[token]
	if !ok || existing.UserID != userID {
		return nil
	}
	delete(s.apiTokens, token)
	return nil
}

// FindUserByAPIToken finds a user by their api token
func (s *DB) FindUserByAPIToken(token string) (*users.User, error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	t, ok := s.apiTokens[token]
	if !ok {
		return nil, users.ErrNotFound
	}
	return s.findUserByID(t.UserID)
}

// ListAPITokensForUserIDs lists the api tokens for these users
func (s *DB) ListAPITokensForUserIDs(userIDs ...string) ([]*users.APIToken, error) {
	var tokens []*users.APIToken
	for _, t := range s.apiTokens {
		for _, userID := range userIDs {
			if t.UserID == userID {
				tokens = append(tokens, t)
				break
			}
		}
	}
	return tokens, nil
}
