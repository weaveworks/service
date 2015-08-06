package main

import (
	"fmt"
	"sort"
	"time"

	"github.com/weaveworks/service/users/names"
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

func (s memoryStorage) ApproveUser(id string) (*User, error) {
	user, err := s.FindUserByID(id)
	if err != nil {
		return nil, err
	}

	o, err := s.createOrganization()
	if err == nil {
		user.ApprovedAt = time.Now().UTC()
		user.OrganizationID = o.ID
		user.OrganizationName = o.Name
		user.ProbeToken = o.ProbeToken
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
	probeToken, err := generateProbeToken()
	if err != nil {
		return nil, err
	}
	o := &Organization{
		ID:         fmt.Sprint(len(s.organizations)),
		Name:       names.Generate(),
		ProbeToken: probeToken,
	}
	s.organizations[o.ID] = o
	return o, nil
}

func (s memoryStorage) FindOrganizationByName(name string) (*Organization, error) {
	for _, o := range s.organizations {
		if o.Name == name {
			return o, nil
		}
	}
	return nil, ErrNotFound
}

func (s memoryStorage) Close() error {
	return nil
}
