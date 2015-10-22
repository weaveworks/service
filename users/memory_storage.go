package main

import (
	"fmt"
	"sort"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type memoryStorage struct {
	users         map[string]*user
	organizations map[string]*organization
}

func newMemoryStorage() *memoryStorage {
	return &memoryStorage{
		users:         make(map[string]*user),
		organizations: make(map[string]*organization),
	}
}

func (s memoryStorage) CreateUser(email string) (*user, error) {
	u := &user{
		ID:        fmt.Sprint(len(s.users)),
		Email:     email,
		CreatedAt: time.Now().UTC(),
	}
	s.users[u.ID] = u
	return u, nil
}

func (s memoryStorage) InviteUser(email, orgName string) (*user, error) {
	var o *organization
	for _, org := range s.organizations {
		if org.Name == orgName {
			o = org
			break
		}
	}
	if o == nil {
		return nil, errNotFound
	}

	u, err := s.FindUserByEmail(email)
	if err == errNotFound {
		u, err = s.CreateUser(email)
	}
	if err != nil {
		return nil, err
	}

	if u.Organization != nil && u.Organization.Name != orgName {
		return nil, errEmailIsTaken
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

func (s memoryStorage) FindUserByID(id string) (*user, error) {
	u, ok := s.users[id]
	if !ok {
		return nil, errNotFound
	}
	return u, nil
}

func (s memoryStorage) FindUserByEmail(email string) (*user, error) {
	for _, user := range s.users {
		if user.Email == email {
			return user, nil
		}
	}
	return nil, errNotFound
}

type usersByCreatedAt []*user

func (u usersByCreatedAt) Len() int           { return len(u) }
func (u usersByCreatedAt) Swap(i, j int)      { u[i], u[j] = u[j], u[i] }
func (u usersByCreatedAt) Less(i, j int) bool { return u[i].CreatedAt.Before(u[j].CreatedAt) }

func (s memoryStorage) ListUsers(fs ...filter) ([]*user, error) {
	users := []*user{}
	for _, user := range s.users {
		ok, err := s.applyFilters(user, fs)
		if err != nil {
			return nil, err
		}
		if ok {
			users = append(users, user)
		}
	}
	sort.Sort(usersByCreatedAt(users))
	return users, nil
}

func (s memoryStorage) ListOrganizationUsers(orgName string) ([]*user, error) {
	users := []*user{}
	for _, user := range s.users {
		if user.Organization != nil && user.Organization.Name == orgName {
			users = append(users, user)
		}
	}
	sort.Sort(usersByCreatedAt(users))
	return users, nil
}

func (s memoryStorage) ApproveUser(id string) (*user, error) {
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
		return errNotFound
	}
	user.Token = string(hashed)
	user.TokenCreatedAt = time.Now().UTC()
	return nil
}

func (s memoryStorage) SetUserFirstLoginAt(id string) error {
	user, ok := s.users[id]
	if !ok {
		return errNotFound
	}
	if user.FirstLoginAt.IsZero() {
		user.FirstLoginAt = time.Now().UTC()
	}
	return nil
}

func (s memoryStorage) createOrganization() (*organization, error) {
	o := &organization{
		ID: fmt.Sprint(len(s.organizations)),
	}
	o.RegenerateName()
	if err := o.RegenerateProbeToken(); err != nil {
		return nil, err
	}
	s.organizations[o.ID] = o
	return o, nil
}

func (s memoryStorage) FindOrganizationByProbeToken(probeToken string) (*organization, error) {
	for _, o := range s.organizations {
		if o.ProbeToken == probeToken {
			if o.FirstProbeUpdateAt.IsZero() {
				o.FirstProbeUpdateAt = time.Now().UTC()
			}
			return o, nil
		}
	}
	return nil, errNotFound
}

func (s memoryStorage) RenameOrganization(oldName, newName string) error {
	for _, o := range s.organizations {
		if o.Name == oldName {
			o.Name = newName
			return nil
		}
	}
	return errNotFound
}

func (s memoryStorage) Close() error {
	return nil
}

func (s memoryStorage) applyFilters(item interface{}, fs []filter) (bool, error) {
	if len(fs) == 0 {
		return true, nil
	}

	match := true
	switch f := fs[0].(type) {
	case usersApprovedFilter:
		u, ok := item.(*user)
		if !ok {
			return false, nil
		}
		if bool(f) {
			match = u.IsApproved()
		} else {
			match = !u.IsApproved()
		}
	case usersOrganizationFilter:
		u, ok := item.(*user)
		if !ok {
			return false, nil
		}
		match = u.Organization != nil && u.Organization.Name == string(f)
	case nil:
		// no-op
	default:
		return false, filterNotImplementedError{f}
	}

	if !match {
		return false, nil
	}
	return s.applyFilters(item, fs[1:])
}
