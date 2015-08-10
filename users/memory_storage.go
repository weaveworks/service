package main

import (
	"fmt"
	"sort"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type memoryStorage struct {
	users         map[string]*User
	organizations map[string]*Organization
}

func newMemoryStorage() *memoryStorage {
	return &memoryStorage{
		users:         make(map[string]*User),
		organizations: make(map[string]*Organization),
	}
}

func (s memoryStorage) CreateUser(email string) (*User, error) {
	u := &User{
		ID:        fmt.Sprint(len(s.users)),
		Email:     email,
		CreatedAt: time.Now().UTC(),
	}
	s.users[u.ID] = u
	return u, nil
}

func (s memoryStorage) InviteUser(email, orgName string) (*User, error) {
	var o *Organization
	for _, org := range s.organizations {
		if org.Name == orgName {
			o = org
			break
		}
	}
	if o == nil {
		return nil, ErrNotFound
	}

	u, err := s.FindUserByEmail(email)
	if err == ErrNotFound {
		u, err = s.CreateUser(email)
	}
	if err != nil {
		return nil, err
	}

	if u.Organization != nil && u.Organization.Name != orgName {
		return nil, ErrEmailIsTaken
	}

	u.ApprovedAt = time.Now().UTC()
	u.Organization = o
	return u, nil
}

func (s memoryStorage) DeleteUser(email string) error {
	for _, user := range s.users {
		if user.Email == email {
			delete(s.users, user.ID)
			break
		}
	}
	return nil
}

func (s memoryStorage) FindUserByID(id string) (*User, error) {
	u, ok := s.users[id]
	if !ok {
		return nil, ErrNotFound
	}
	return u, nil
}

func (s memoryStorage) FindUserByEmail(email string) (*User, error) {
	for _, user := range s.users {
		if user.Email == email {
			return user, nil
		}
	}
	return nil, ErrNotFound
}

type usersByCreatedAt []*User

func (u usersByCreatedAt) Len() int           { return len(u) }
func (u usersByCreatedAt) Swap(i, j int)      { u[i], u[j] = u[j], u[i] }
func (u usersByCreatedAt) Less(i, j int) bool { return u[i].CreatedAt.Before(u[j].CreatedAt) }

func (s memoryStorage) ListUnapprovedUsers() ([]*User, error) {
	users := []*User{}
	for _, user := range s.users {
		if user.ApprovedAt.IsZero() {
			users = append(users, user)
		}
	}
	sort.Sort(usersByCreatedAt(users))
	return users, nil
}

func (s memoryStorage) ListOrganizationUsers(orgName string) ([]*User, error) {
	users := []*User{}
	for _, user := range s.users {
		if user.Organization != nil && user.Organization.Name == orgName {
			users = append(users, user)
		}
	}
	sort.Sort(usersByCreatedAt(users))
	return users, nil
}

func (s memoryStorage) ApproveUser(id string) (*User, error) {
	user, err := s.FindUserByID(id)
	if err != nil {
		return nil, err
	}

	if !user.ApprovedAt.IsZero() {
		return user, nil
	}

	if o, err := s.createOrganization(); err == nil {
		user.ApprovedAt = time.Now().UTC()
		user.Organization = o
	}
	return user, err
}

func (s memoryStorage) SetUserToken(id, token string) error {
	var hashed []byte
	if token != "" {
		var err error
		hashed, err = bcrypt.GenerateFromPassword([]byte(token), passwordHashingCost)
		if err != nil {
			return err
		}
	}
	user, ok := s.users[id]
	if !ok {
		return ErrNotFound
	}
	user.Token = string(hashed)
	user.TokenCreatedAt = time.Now().UTC()
	return nil
}

func (s memoryStorage) createOrganization() (*Organization, error) {
	o := &Organization{
		ID: fmt.Sprint(len(s.organizations)),
	}
	o.RegenerateName()
	if err := o.RegenerateProbeToken(); err != nil {
		return nil, err
	}
	s.organizations[o.ID] = o
	return o, nil
}

func (s memoryStorage) FindOrganizationByProbeToken(probeToken string) (*Organization, error) {
	for _, o := range s.organizations {
		if o.ProbeToken == probeToken {
			if o.FirstProbeUpdateAt.IsZero() {
				o.FirstProbeUpdateAt = time.Now().UTC()
			}
			return o, nil
		}
	}
	return nil, ErrNotFound
}

func (s memoryStorage) Close() error {
	return nil
}
