package storage

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/externalIDs"
	"github.com/weaveworks/service/users/login"
)

type memoryStorage struct {
	users         map[string]*users.User
	organizations map[string]*users.Organization
	memberships   map[string][]string
	logins        map[string]*login.Login
	apiTokens     map[string]*users.APIToken
	mtx           sync.Mutex
}

func newMemoryStorage() *memoryStorage {
	return &memoryStorage{
		users:         make(map[string]*users.User),
		organizations: make(map[string]*users.Organization),
		memberships:   make(map[string][]string),
		logins:        make(map[string]*login.Login),
		apiTokens:     make(map[string]*users.APIToken),
	}
}

func (s *memoryStorage) CreateUser(email string) (*users.User, error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	return s.createUser(email)
}

func (s *memoryStorage) createUser(email string) (*users.User, error) {
	u := &users.User{
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

func (s *memoryStorage) DetachLoginFromUser(userID, provider string) error {
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

func (s *memoryStorage) CreateAPIToken(userID, description string) (*users.APIToken, error) {
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

func (s *memoryStorage) DeleteAPIToken(userID, token string) error {
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

func (s *memoryStorage) InviteUser(email, orgExternalID string) (*users.User, bool, error) {
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

func (s *memoryStorage) RemoveUserFromOrganization(orgExternalID, email string) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	o, err := s.findOrganizationByExternalID(orgExternalID)
	if err != nil && err != users.ErrNotFound {
		return nil
	}
	u, err := s.findUserByEmail(email)
	if err != nil && err != users.ErrNotFound {
		return nil
	}

	memberships, ok := s.memberships[o.ID]
	if !ok {
		return nil
	}
	var newMemberships []string
	for _, m := range memberships {
		if m != u.ID {
			newMemberships = append(newMemberships, m)
		}
	}
	s.memberships[o.ID] = newMemberships
	return nil
}

func (s *memoryStorage) FindUserByID(id string) (*users.User, error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	return s.findUserByID(id)
}

func (s *memoryStorage) findUserByID(id string) (*users.User, error) {
	u, ok := s.users[id]
	if !ok {
		return nil, users.ErrNotFound
	}
	return u, nil
}

func (s *memoryStorage) FindUserByEmail(email string) (*users.User, error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	return s.findUserByEmail(email)
}

func (s *memoryStorage) findUserByEmail(email string) (*users.User, error) {
	for _, user := range s.users {
		if user.Email == email {
			return user, nil
		}
	}
	return nil, users.ErrNotFound
}

func (s *memoryStorage) FindUserByLogin(provider, providerID string) (*users.User, error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	return s.findUserByLogin(provider, providerID)
}

func (s *memoryStorage) findUserByLogin(provider, providerID string) (*users.User, error) {
	for _, l := range s.logins {
		if l.Provider == provider && l.ProviderID == providerID {
			return s.findUserByID(l.UserID)
		}
	}
	return nil, users.ErrNotFound
}

func (s *memoryStorage) FindUserByAPIToken(token string) (*users.User, error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	t, ok := s.apiTokens[token]
	if !ok {
		return nil, users.ErrNotFound
	}
	return s.findUserByID(t.UserID)
}

func (s *memoryStorage) UserIsMemberOf(userID, orgExternalID string) (bool, error) {
	o, err := s.findOrganizationByExternalID(orgExternalID)
	if err == users.ErrNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	for _, m := range s.memberships[o.ID] {
		if m == userID {
			return true, nil
		}
	}
	return false, nil
}

type usersByCreatedAt []*users.User

func (u usersByCreatedAt) Len() int           { return len(u) }
func (u usersByCreatedAt) Swap(i, j int)      { u[i], u[j] = u[j], u[i] }
func (u usersByCreatedAt) Less(i, j int) bool { return u[i].CreatedAt.Before(u[j].CreatedAt) }

func (s *memoryStorage) ListUsers() ([]*users.User, error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	users := []*users.User{}
	for _, user := range s.users {
		users = append(users, user)
	}
	sort.Sort(usersByCreatedAt(users))
	return users, nil
}

func (s *memoryStorage) ListOrganizationUsers(orgExternalID string) ([]*users.User, error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	o, err := s.findOrganizationByExternalID(orgExternalID)
	if err != nil {
		return nil, err
	}
	var users []*users.User
	for _, m := range s.memberships[o.ID] {
		u, err := s.findUserByID(m)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	sort.Sort(usersByCreatedAt(users))
	return users, nil
}

type organizationsByCreatedAt []*users.Organization

func (o organizationsByCreatedAt) Len() int           { return len(o) }
func (o organizationsByCreatedAt) Swap(i, j int)      { o[i], o[j] = o[j], o[i] }
func (o organizationsByCreatedAt) Less(i, j int) bool { return o[i].CreatedAt.Before(o[j].CreatedAt) }

func (s *memoryStorage) ListOrganizationsForUserIDs(userIDs ...string) ([]*users.Organization, error) {
	orgIDs := map[string]struct{}{}
	checkOrg := func(orgID string, members []string) {
		for _, m := range members {
			for _, userID := range userIDs {
				if m == userID {
					orgIDs[orgID] = struct{}{}
					return
				}
			}
		}
	}
	for orgID, members := range s.memberships {
		checkOrg(orgID, members)
	}
	var orgs []*users.Organization
	for orgID := range orgIDs {
		o, ok := s.organizations[orgID]
		if !ok {
			return nil, users.ErrNotFound
		}
		orgs = append(orgs, o)
	}
	sort.Sort(organizationsByCreatedAt(orgs))
	return orgs, nil
}

func (s *memoryStorage) ListLoginsForUserIDs(userIDs ...string) ([]*login.Login, error) {
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

func (s *memoryStorage) ListAPITokensForUserIDs(userIDs ...string) ([]*users.APIToken, error) {
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

func (s *memoryStorage) ApproveUser(id string) (*users.User, error) {
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
		return users.ErrNotFound
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
		hashed, err = bcrypt.GenerateFromPassword([]byte(token), PasswordHashingCost)
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

func (s *memoryStorage) SetUserFirstLoginAt(id string) error {
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

func (s *memoryStorage) GenerateOrganizationExternalID() (string, error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	var externalID string
	for {
		externalID = externalIDs.Generate()
		_, err := s.findOrganizationByExternalID(externalID)
		if err != nil && err != users.ErrNotFound {
			return "", err
		}
		break
	}
	return externalID, nil
}

func (s *memoryStorage) findOrganizationByExternalID(externalID string) (*users.Organization, error) {
	for _, o := range s.organizations {
		if strings.ToLower(o.ExternalID) == strings.ToLower(externalID) {
			return o, nil
		}
	}
	return nil, users.ErrNotFound
}

func (s *memoryStorage) CreateOrganization(ownerID, externalID, name string) (*users.Organization, error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	if _, err := s.findUserByID(ownerID); err != nil {
		return nil, err
	}
	o := &users.Organization{
		ID:         fmt.Sprint(len(s.organizations)),
		ExternalID: externalID,
		Name:       name,
		CreatedAt:  time.Now().UTC(),
	}
	if err := o.Valid(); err != nil {
		return nil, err
	}
	if exists, err := s.organizationExists(o.ExternalID); err != nil {
		return nil, err
	} else if exists {
		return nil, users.ErrOrgExternalIDIsTaken
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
	s.memberships[o.ID] = []string{ownerID}
	return o, nil
}

func (s *memoryStorage) FindOrganizationByProbeToken(probeToken string) (*users.Organization, error) {
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
	return nil, users.ErrNotFound
}

func (s *memoryStorage) RenameOrganization(externalID, name string) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	if err := (&users.Organization{ExternalID: externalID, Name: name}).Valid(); err != nil {
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
	if _, err := s.findOrganizationByExternalID(externalID); err == users.ErrNotFound {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

func (s *memoryStorage) GetOrganizationName(externalID string) (string, error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
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
	if err == users.ErrNotFound {
		return nil
	}
	if err != nil {
		return err
	}
	delete(s.organizations, o.ID)
	delete(s.memberships, o.ID)
	return nil
}

func (s *memoryStorage) AddFeatureFlag(externalID string, featureFlag string) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	o, err := s.findOrganizationByExternalID(externalID)
	if err == users.ErrNotFound {
		return nil
	}
	if err != nil {
		return err
	}
	o.FeatureFlags = append(o.FeatureFlags, featureFlag)
	return nil
}

func (s *memoryStorage) Close() error {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	return nil
}
