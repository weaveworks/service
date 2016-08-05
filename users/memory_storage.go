package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/weaveworks/service/users/externalIDs"
	"github.com/weaveworks/service/users/login"
)

type memoryStorage struct {
	users         map[string]*user
	organizations map[string]*organization
	mtx           sync.Mutex
}

func newMemoryStorage() *memoryStorage {
	return &memoryStorage{
		users:         make(map[string]*user),
		organizations: make(map[string]*organization),
	}
}

func (s *memoryStorage) CreateUser(email string) (*user, error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	return s.createUser(email)
}

func (s *memoryStorage) createUser(email string) (*user, error) {
	u := &user{
		ID:        fmt.Sprint(len(s.users)),
		Email:     strings.ToLower(email),
		CreatedAt: time.Now().UTC(),
	}
	s.users[u.ID] = u
	return u, nil
}

func (s *memoryStorage) AddLoginToUser(userID, provider, providerID string, session json.RawMessage) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	u, err := s.findUserByID(userID)
	if err != nil {
		return err
	}

	// Remove any existing links to other user accounts
	createdAt := time.Now().UTC()
	for _, user := range s.users {
		var newLogins []*login.Login
		for _, a := range user.Logins {
			if a.Provider == provider && a.ProviderID == providerID {
				createdAt = a.CreatedAt
			} else {
				newLogins = append(newLogins, a)
			}
		}
		user.Logins = newLogins
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

func (s *memoryStorage) DetachLoginFromUser(userID, provider string) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	u, err := s.findUserByID(userID)
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

func (s *memoryStorage) InviteUser(email, orgExternalID string) (*user, error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	o, err := s.findOrganizationByExternalID(orgExternalID)
	if err != nil {
		return nil, err
	}

	u, err := s.findUserByEmail(email)
	if err == errNotFound {
		u, err = s.createUser(email)
	}
	if err != nil {
		return nil, err
	}

	if u.HasOrganization(orgExternalID) {
		return u, nil
	}
	u.Organizations = append(u.Organizations, o)
	return u, nil
}

func (s *memoryStorage) RemoveUserFromOrganization(orgExternalID, email string) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	for _, user := range s.users {
		if user.Email == email {
			var newOrganizations []*organization
			for _, o := range user.Organizations {
				if strings.ToLower(orgExternalID) != strings.ToLower(o.ExternalID) {
					newOrganizations = append(newOrganizations, o)
				}
			}
			user.Organizations = newOrganizations
			break
		}
	}
	return nil
}

func (s *memoryStorage) FindUserByID(id string) (*user, error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	return s.findUserByID(id)
}

func (s *memoryStorage) findUserByID(id string) (*user, error) {
	u, ok := s.users[id]
	if !ok {
		return nil, errNotFound
	}
	return u, nil
}

func (s *memoryStorage) FindUserByEmail(email string) (*user, error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	return s.findUserByEmail(email)
}

func (s *memoryStorage) findUserByEmail(email string) (*user, error) {
	for _, user := range s.users {
		if user.Email == email {
			return user, nil
		}
	}
	return nil, errNotFound
}

func (s *memoryStorage) FindUserByLogin(provider, providerID string) (*user, error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
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

func (s *memoryStorage) ListUsers(fs ...filter) ([]*user, error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	users := []*user{}
	for _, user := range s.users {
		if s.applyFilters(user, fs) {
			users = append(users, user)
		}
	}
	sort.Sort(usersByCreatedAt(users))
	return users, nil
}

func (s *memoryStorage) ListOrganizationUsers(orgExternalID string) ([]*user, error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	users := []*user{}
	for _, user := range s.users {
		for _, org := range user.Organizations {
			if strings.ToLower(org.ExternalID) == strings.ToLower(orgExternalID) {
				users = append(users, user)
				break
			}
		}
	}
	sort.Sort(usersByCreatedAt(users))
	return users, nil
}

func (s *memoryStorage) ApproveUser(id string) (*user, error) {
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
func (s *memoryStorage) SetUserAdmin(id string, value bool) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	user, ok := s.users[id]
	if !ok {
		return errNotFound
	}
	user.Admin = value
	return nil
}

func (s *memoryStorage) SetUserToken(id, token string) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()
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

func (s *memoryStorage) SetUserFirstLoginAt(id string) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	user, ok := s.users[id]
	if !ok {
		return errNotFound
	}
	if user.FirstLoginAt.IsZero() {
		user.FirstLoginAt = time.Now().UTC()
	}
	return nil
}

func (s *memoryStorage) GenerateOrganizationExternalID() (string, error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	var externalID string
	for {
		externalID = externalIDs.Generate()
		_, err := s.findOrganizationByExternalID(externalID)
		if err != nil && err != errNotFound {
			return "", err
		}
		break
	}
	return externalID, nil
}

func (s *memoryStorage) findOrganizationByExternalID(externalID string) (*organization, error) {
	for _, o := range s.organizations {
		if strings.ToLower(o.ExternalID) == strings.ToLower(externalID) {
			return o, nil
		}
	}
	return nil, errNotFound
}

func (s *memoryStorage) CreateOrganization(ownerID, externalID, name string) (*organization, error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	user, err := s.findUserByID(ownerID)
	if err != nil {
		return nil, err
	}
	o := &organization{
		ID:         fmt.Sprint(len(s.organizations)),
		ExternalID: externalID,
		Name:       name,
		CreatedAt:  time.Now().UTC(),
	}
	if err := o.valid(); err != nil {
		return nil, err
	}
	if exists, err := s.organizationExists(o.ExternalID); err != nil {
		return nil, err
	} else if exists {
		return nil, errOrgExternalIDIsTaken
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

func (s *memoryStorage) FindOrganizationByProbeToken(probeToken string) (*organization, error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
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

func (s *memoryStorage) RenameOrganization(externalID, name string) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	if err := (&organization{ExternalID: externalID, Name: name}).valid(); err != nil {
		return err
	}

	o, err := s.findOrganizationByExternalID(externalID)
	if err != nil {
		return err
	}

	o.Name = name
	return nil
}

func (s *memoryStorage) OrganizationExists(externalID string) (bool, error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	return s.organizationExists(externalID)
}

func (s *memoryStorage) organizationExists(externalID string) (bool, error) {
	if _, err := s.findOrganizationByExternalID(externalID); err == errNotFound {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

func (s *memoryStorage) GetOrganizationName(externalID string) (string, error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	return s.getOrganizationName(externalID)
}

func (s *memoryStorage) getOrganizationName(externalID string) (string, error) {
	o, err := s.findOrganizationByExternalID(externalID)
	if err != nil {
		return "", err
	}
	return o.Name, nil
}

func (s *memoryStorage) DeleteOrganization(externalID string) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	o, err := s.findOrganizationByExternalID(externalID)
	if err == errNotFound {
		return nil
	}
	if err != nil {
		return err
	}
	delete(s.organizations, o.ID)

	for _, user := range s.users {
		var newOrganizations []*organization
		for _, org := range user.Organizations {
			if org.ID != o.ID {
				newOrganizations = append(newOrganizations, org)
			}
		}
		user.Organizations = newOrganizations
	}
	return nil
}

func (s *memoryStorage) Close() error {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	return nil
}

func (s *memoryStorage) applyFilters(item interface{}, fs []filter) bool {
	for _, f := range fs {
		if !f.Item(item) {
			return false
		}
	}
	return true
}
