package memory

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/db"
	"github.com/weaveworks/service/users/login"
)

func (s *memoryDB) CreateUser(email string) (*users.User, error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	return s.createUser(email)
}

func (s *memoryDB) createUser(email string) (*users.User, error) {
	u := &users.User{
		ID:        fmt.Sprint(len(s.users)),
		Email:     strings.ToLower(email),
		CreatedAt: time.Now().UTC(),
	}
	s.users[u.ID] = u
	return u, nil
}

func (s *memoryDB) AddLoginToUser(userID, provider, providerID string, session json.RawMessage) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	if _, err := s.findUserByID(userID); err != nil {
		return err
	}

	// Check if this login is attached to another user
	existing, err := s.findUserByLogin(provider, providerID)
	if err == nil && existing.ID != userID {
		return users.AlreadyAttachedError{ID: existing.ID, Email: existing.Email}
	}

	// Add it to this one (updating session if needed).
	s.logins[fmt.Sprintf("%s-%s", provider, providerID)] = &login.Login{
		UserID:     userID,
		Provider:   provider,
		ProviderID: providerID,
		Session:    session,
		CreatedAt:  time.Now().UTC(),
	}
	return nil
}

func (s *memoryDB) DetachLoginFromUser(userID, provider string) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	_, err := s.findUserByID(userID)
	if err == users.ErrNotFound {
		return nil
	}
	if err != nil {
		return err
	}
	newLogins := make(map[string]*login.Login)
	for k, v := range s.logins {
		if v.UserID != userID || v.Provider != provider {
			newLogins[k] = v
		}
	}
	s.logins = newLogins
	return nil
}

func (s *memoryDB) InviteUser(email, orgExternalID string) (*users.User, bool, error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	created := false
	o, err := s.findOrganizationByExternalID(orgExternalID)
	if err != nil {
		return nil, false, err
	}

	u, err := s.findUserByEmail(email)
	if err == users.ErrNotFound {
		u, err = s.createUser(email)
		created = true
	}
	if err != nil {
		return nil, false, err
	}

	isMember, err := s.UserIsMemberOf(u.ID, orgExternalID)
	if err != nil {
		return nil, false, err
	}
	if isMember {
		return u, false, nil
	}
	s.memberships[o.ID] = append(s.memberships[o.ID], u.ID)
	return u, created, nil
}

func (s *memoryDB) FindUserByID(id string) (*users.User, error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	return s.findUserByID(id)
}

func (s *memoryDB) findUserByID(id string) (*users.User, error) {
	u, ok := s.users[id]
	if !ok {
		return nil, users.ErrNotFound
	}
	return u, nil
}

func (s *memoryDB) FindUserByEmail(email string) (*users.User, error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	return s.findUserByEmail(email)
}

func (s *memoryDB) findUserByEmail(email string) (*users.User, error) {
	for _, user := range s.users {
		if user.Email == email {
			return user, nil
		}
	}
	return nil, users.ErrNotFound
}

func (s *memoryDB) FindUserByLogin(provider, providerID string) (*users.User, error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	return s.findUserByLogin(provider, providerID)
}

func (s *memoryDB) findUserByLogin(provider, providerID string) (*users.User, error) {
	for _, l := range s.logins {
		if l.Provider == provider && l.ProviderID == providerID {
			return s.findUserByID(l.UserID)
		}
	}
	return nil, users.ErrNotFound
}

type usersByCreatedAt []*users.User

func (u usersByCreatedAt) Len() int           { return len(u) }
func (u usersByCreatedAt) Swap(i, j int)      { u[i], u[j] = u[j], u[i] }
func (u usersByCreatedAt) Less(i, j int) bool { return u[i].CreatedAt.Before(u[j].CreatedAt) }

func (s *memoryDB) ListUsers() ([]*users.User, error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	users := []*users.User{}
	for _, user := range s.users {
		users = append(users, user)
	}
	sort.Sort(usersByCreatedAt(users))
	return users, nil
}

func (s *memoryDB) ListLoginsForUserIDs(userIDs ...string) ([]*login.Login, error) {
	var logins []*login.Login
	for _, l := range s.logins {
		for _, userID := range userIDs {
			if l.UserID == userID {
				logins = append(logins, l)
				break
			}
		}
	}
	return logins, nil
}

func (s *memoryDB) ApproveUser(id string) (*users.User, error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	user, err := s.findUserByID(id)
	if err != nil {
		return nil, err
	}

	if !user.ApprovedAt.IsZero() {
		return user, nil
	}

	user.ApprovedAt = time.Now().UTC()
	return user, err
}

// Set the admin flag of a user
func (s *memoryDB) SetUserAdmin(id string, value bool) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	user, ok := s.users[id]
	if !ok {
		return users.ErrNotFound
	}
	user.Admin = value
	return nil
}

func (s *memoryDB) SetUserToken(id, token string) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	var hashed []byte
	if token != "" {
		var err error
		hashed, err = bcrypt.GenerateFromPassword([]byte(token), db.PasswordHashingCost)
		if err != nil {
			return err
		}
	}
	user, ok := s.users[id]
	if !ok {
		return users.ErrNotFound
	}
	user.Token = string(hashed)
	user.TokenCreatedAt = time.Now().UTC()
	return nil
}

func (s *memoryDB) SetUserFirstLoginAt(id string) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	user, ok := s.users[id]
	if !ok {
		return users.ErrNotFound
	}
	if user.FirstLoginAt.IsZero() {
		user.FirstLoginAt = time.Now().UTC()
	}
	return nil
}
