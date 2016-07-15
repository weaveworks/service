package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/weaveworks/service/users/login"
	"github.com/weaveworks/service/users/names"
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
		Email:     strings.ToLower(email),
		CreatedAt: time.Now().UTC(),
	}
	s.users[u.ID] = u
	return u, nil
}

func (s memoryStorage) AddLoginToUser(userID, provider, providerID string, session json.RawMessage) error {
	u, err := s.FindUserByID(userID)
	if err != nil {
		return err
	}

	// Remove any existing links to other user accounts
	createdAt := time.Now().UTC()
	for _, user := range s.users {
		for i, a := range user.Logins {
			if a.Provider == provider && a.ProviderID == providerID {
				createdAt = a.CreatedAt
				user.Logins = append(user.Logins[:i], user.Logins[i+1:]...)
			}
		}
	}

	// Add it to this one (updating session if needed).
	u.Logins = append(u.Logins, &login.Login{
		UserID:     userID,
		Provider:   provider,
		ProviderID: providerID,
		Session:    session,
		CreatedAt:  createdAt,
	})
	sort.Sort(login.LoginsByProvider(u.Logins))
	return nil
}

func (s memoryStorage) DetachLoginFromUser(userID, provider string) error {
	u, err := s.FindUserByID(userID)
	if err == errNotFound {
		return nil
	}
	if err != nil {
		return err
	}
	newLogins := []*login.Login{}
	for _, a := range u.Logins {
		if a.Provider != provider {
			newLogins = append(newLogins, a)
		}
	}
	u.Logins = newLogins
	return nil
}

func (s memoryStorage) InviteUser(email, orgName string) (*user, error) {
	o, err := s.findOrganizationByName(orgName)
	if err != nil {
		return nil, err
	}

	u, err := s.FindUserByEmail(email)
	if err == errNotFound {
		u, err = s.CreateUser(email)
	}
	if err != nil {
		return nil, err
	}

	switch len(u.Organizations) {
	case 0:
		u.Organizations = append(u.Organizations, o)
		return u, nil
	case 1:
		if strings.ToLower(u.Organizations[0].Name) == strings.ToLower(orgName) {
			return u, nil
		}
	}
	return nil, errEmailIsTaken
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

func (s memoryStorage) FindUserByLogin(provider, providerID string) (*user, error) {
	for _, user := range s.users {
		for _, a := range user.Logins {
			if a.Provider == provider && a.ProviderID == providerID {
				return user, nil
			}
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
		if s.applyFilters(user, fs) {
			users = append(users, user)
		}
	}
	sort.Sort(usersByCreatedAt(users))
	return users, nil
}

func (s memoryStorage) ListOrganizationUsers(orgName string) ([]*user, error) {
	users := []*user{}
	for _, user := range s.users {
		for _, org := range user.Organizations {
			if strings.ToLower(org.Name) == strings.ToLower(orgName) {
				users = append(users, user)
				break
			}
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

	user.ApprovedAt = time.Now().UTC()
	return user, err
}

// Set the admin flag of a user
func (s memoryStorage) SetUserAdmin(id string, value bool) error {
	user, ok := s.users[id]
	if !ok {
		return errNotFound
	}
	user.Admin = value
	return nil
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

func (s memoryStorage) GenerateOrganizationName() (string, error) {
	var name string
	for {
		name = names.Generate()
		_, err := s.findOrganizationByName(name)
		if err != nil && err != errNotFound {
			return "", err
		}
		break
	}
	return name, nil
}

func (s memoryStorage) findOrganizationByName(name string) (*organization, error) {
	for _, o := range s.organizations {
		if strings.ToLower(o.Name) == strings.ToLower(name) {
			return o, nil
		}
	}
	return nil, errNotFound
}

func (s memoryStorage) CreateOrganization(ownerID, name, label string) (*organization, error) {
	user, err := s.FindUserByID(ownerID)
	if err != nil {
		return nil, err
	}
	o := &organization{
		ID:        fmt.Sprint(len(s.organizations)),
		Name:      name,
		Label:     label,
		CreatedAt: time.Now().UTC(),
	}
	if err := o.Valid(); err != nil {
		return nil, err
	}
	if exists, err := s.OrganizationExists(o.Name); err != nil {
		return nil, err
	} else if exists {
		return nil, errOrgNameIsTaken
	}
	for exists := o.ProbeToken == ""; exists; {
		if err := o.RegenerateProbeToken(); err != nil {
			return nil, err
		}
		exists = false
		for _, org := range s.organizations {
			if org.ProbeToken == o.ProbeToken {
				exists = true
				break
			}
		}
	}
	s.organizations[o.ID] = o
	user.Organizations = append(user.Organizations, o)
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

func (s memoryStorage) RelabelOrganization(name, label string) error {
	if err := (&organization{Name: name, Label: label}).Valid(); err != nil {
		return err
	}

	o, err := s.findOrganizationByName(name)
	if err != nil {
		return err
	}

	o.Label = label
	return nil
}

func (s memoryStorage) OrganizationExists(name string) (bool, error) {
	if _, err := s.findOrganizationByName(name); err == errNotFound {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

func (s memoryStorage) Close() error {
	return nil
}

func (s memoryStorage) applyFilters(item interface{}, fs []filter) bool {
	for _, f := range fs {
		if !f.Item(item) {
			return false
		}
	}
	return true
}
