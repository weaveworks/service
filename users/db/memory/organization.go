package memory

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/externalIDs"
)

// RemoveUserFromOrganization removes the user from the organiation. If they
// are not a member, this is a noop.
func (s *DB) RemoveUserFromOrganization(orgExternalID, email string) error {
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

// UserIsMemberOf checks if the user is a member of the organization
func (s *DB) UserIsMemberOf(userID, orgExternalID string) (bool, error) {
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

// ListOrganizations lists organizations
func (s *DB) ListOrganizations() ([]*users.Organization, error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	orgs := []*users.Organization{}
	for _, org := range s.organizations {
		orgs = append(orgs, org)
	}
	sort.Sort(organizationsByCreatedAt(orgs))
	return orgs, nil
}

// ListOrganizationUsers lists all the users in an organization
func (s *DB) ListOrganizationUsers(orgExternalID string) ([]*users.User, error) {
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

// ListOrganizationsForUserIDs lists the organizations these users belong to
func (s *DB) ListOrganizationsForUserIDs(userIDs ...string) ([]*users.Organization, error) {
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

// GenerateOrganizationExternalID returns an available organization external
// id, e.g. creaky-door-97
// TODO: There is a known issue, where as we fill up the database this will
// gradually slow down (since the algorithm is quite naive). We should fix it
// eventually.
func (s *DB) GenerateOrganizationExternalID() (string, error) {
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

func (s *DB) findOrganizationByExternalID(externalID string) (*users.Organization, error) {
	for _, o := range s.organizations {
		if strings.ToLower(o.ExternalID) == strings.ToLower(externalID) {
			return o, nil
		}
	}
	return nil, users.ErrNotFound
}

// CreateOrganization creates a new organization owned by the user
func (s *DB) CreateOrganization(ownerID, externalID, name string) (*users.Organization, error) {
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

// FindOrganizationByProbeToken looks up the organization matching a given
// probe token.
func (s *DB) FindOrganizationByProbeToken(probeToken string) (*users.Organization, error) {
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

// RenameOrganization changes an organization's user-settable name
func (s *DB) RenameOrganization(externalID, name string) error {
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

// OrganizationExists just returns a simple bool checking if an organization
// exists
func (s *DB) OrganizationExists(externalID string) (bool, error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	return s.organizationExists(externalID)
}

func (s *DB) organizationExists(externalID string) (bool, error) {
	if _, err := s.findOrganizationByExternalID(externalID); err == users.ErrNotFound {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

// GetOrganizationName gets the name of an organization from it's external ID.
func (s *DB) GetOrganizationName(externalID string) (string, error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	o, err := s.findOrganizationByExternalID(externalID)
	if err != nil {
		return "", err
	}
	return o.Name, nil
}

// DeleteOrganization deletes an organization
func (s *DB) DeleteOrganization(externalID string) error {
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

// AddFeatureFlag adds a new feature flag to an organization.
func (s *DB) AddFeatureFlag(externalID string, featureFlag string) error {
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
