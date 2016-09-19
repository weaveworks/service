package memory

import (
	"fmt"
	"time"

	"github.com/weaveworks/service/users"
)

func (s *memoryDB) CreateAPIToken(userID, description string) (*users.APIToken, error) {
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

func (s *memoryDB) DeleteAPIToken(userID, token string) error {
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

func (s *memoryDB) FindUserByAPIToken(token string) (*users.User, error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	t, ok := s.apiTokens[token]
	if !ok {
		return nil, users.ErrNotFound
	}
	return s.findUserByID(t.UserID)
}

func (s *memoryDB) ListAPITokensForUserIDs(userIDs ...string) ([]*users.APIToken, error) {
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
