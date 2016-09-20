package memory

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/login"
)

// CreateUser creates a new user with the given email.
func (s *DB) CreateUser(email string) (*users.User, error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	return s.createUser(email)
}

func (s *DB) createUser(email string) (*users.User, error) {
	u := &users.User{
		ID:        fmt.Sprint(len(s.users)),
		Email:     strings.ToLower(email),
		CreatedAt: time.Now().UTC(),
	}
	s.users[u.ID] = u
	return u, nil
}

// AddLoginToUser adds the given login to the specified user. If it is already
// attached elsewhere, this will error.
func (s *DB) AddLoginToUser(userID, provider, providerID string, session json.RawMessage) error {
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

// DetachLoginFromUser detaches the specified login from a user. e.g. if you
// want to attach it to a different user, do this first.
func (s *DB) DetachLoginFromUser(userID, provider string) error {
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

// InviteUser invites the user, to join the organization. If they are already a
// member this is a noop.
func (s *DB) InviteUser(email, orgExternalID string) (*users.User, bool, error) {
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

// FindUserByID finds the user by id
func (s *DB) FindUserByID(id string) (*users.User, error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	return s.findUserByID(id)
}

func (s *DB) findUserByID(id string) (*users.User, error) {
	u, ok := s.users[id]
	if !ok {
		return nil, users.ErrNotFound
	}
	return u, nil
}

// FindUserByEmail finds the user by email
func (s *DB) FindUserByEmail(email string) (*users.User, error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	return s.findUserByEmail(email)
}

func (s *DB) findUserByEmail(email string) (*users.User, error) {
	for _, user := range s.users {
		if user.Email == email {
			return user, nil
		}
	}
	return nil, users.ErrNotFound
}

// FindUserByLogin finds the user by login
func (s *DB) FindUserByLogin(provider, providerID string) (*users.User, error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	return s.findUserByLogin(provider, providerID)
}

func (s *DB) findUserByLogin(provider, providerID string) (*users.User, error) {
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

// ListUsers lists users
func (s *DB) ListUsers() ([]*users.User, error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	users := []*users.User{}
	for _, user := range s.users {
		users = append(users, user)
	}
	sort.Sort(usersByCreatedAt(users))
	return users, nil
}

// ListLoginsForUserIDs lists the logins for these users
func (s *DB) ListLoginsForUserIDs(userIDs ...string) ([]*login.Login, error) {
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

// ApproveUser approves a user. Sort of deprecated, as all users are
// auto-approved now.
func (s *DB) ApproveUser(id string) (*users.User, error) {
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

// SetUserAdmin sets the admin flag of a user
func (s *DB) SetUserAdmin(id string, value bool) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	user, ok := s.users[id]
	if !ok {
		return users.ErrNotFound
	}
	user.Admin = value
	return nil
}

// SetUserToken updates the user's login token
func (s *DB) SetUserToken(id, token string) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	var hashed []byte
	if token != "" {
		var err error
		hashed, err = bcrypt.GenerateFromPassword([]byte(token), s.passwordHashingCost)
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

// SetUserFirstLoginAt is called the first time a user logs in, to set their
// first_login_at field.
func (s *DB) SetUserFirstLoginAt(id string) error {
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
