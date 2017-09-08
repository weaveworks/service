package memory

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/lib/pq"
	"golang.org/x/net/context"

	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/db/filter"
	"github.com/weaveworks/service/users/externalIDs"
)

// RemoveUserFromOrganization removes the user from the organiation. If they
// are not a member, this is a noop.
func (d *DB) RemoveUserFromOrganization(_ context.Context, orgExternalID, email string) error {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	o, err := d.findOrganizationByExternalID(orgExternalID)
	if err != nil && err != users.ErrNotFound {
		return nil
	}
	u, err := d.findUserByEmail(email)
	if err != nil && err != users.ErrNotFound {
		return nil
	}

	memberships, ok := d.memberships[o.ID]
	if !ok {
		return nil
	}
	var newMemberships []string
	for _, m := range memberships {
		if m != u.ID {
			newMemberships = append(newMemberships, m)
		}
	}
	d.memberships[o.ID] = newMemberships
	return nil
}

// UserIsMemberOf checks if the user is a member of the organization
func (d *DB) UserIsMemberOf(_ context.Context, userID, orgExternalID string) (bool, error) {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	return d.userIsMemberOf(userID, orgExternalID)
}

func (d *DB) userIsMemberOf(userID, orgExternalID string) (bool, error) {
	o, err := d.findOrganizationByExternalID(orgExternalID)
	if err == users.ErrNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	for _, m := range d.memberships[o.ID] {
		if m == userID {
			return true, nil
		}
	}
	return false, nil
}

// ListOrganizations lists organizations
func (d *DB) ListOrganizations(_ context.Context, f filter.Organization) ([]*users.Organization, error) {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	orgs := []*users.Organization{}
	for _, org := range d.organizations {
		if strings.Contains(org.Name, f.Query) {
			orgs = append(orgs, org)
		}
	}
	sort.Sort(organizationsByCreatedAt(orgs))
	return orgs, nil
}

// ListOrganizationUsers lists all the users in an organization
func (d *DB) ListOrganizationUsers(_ context.Context, orgExternalID string) ([]*users.User, error) {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	o, err := d.findOrganizationByExternalID(orgExternalID)
	if err != nil {
		return nil, err
	}
	var users []*users.User
	for _, m := range d.memberships[o.ID] {
		u, err := d.findUserByID(m)
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
func (o organizationsByCreatedAt) Less(i, j int) bool { return o[i].CreatedAt.After(o[j].CreatedAt) }

// ListOrganizationsForUserIDs lists the organizations these users belong to
func (d *DB) ListOrganizationsForUserIDs(_ context.Context, userIDs ...string) ([]*users.Organization, error) {
	d.mtx.Lock()
	defer d.mtx.Unlock()
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
	for orgID, members := range d.memberships {
		checkOrg(orgID, members)
	}
	var orgs []*users.Organization
	for orgID := range orgIDs {
		o, ok := d.organizations[orgID]
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
func (d *DB) GenerateOrganizationExternalID(_ context.Context) (string, error) {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	var externalID string
	for {
		externalID = externalIDs.Generate()
		_, err := d.findOrganizationByExternalID(externalID)
		if err != nil && err != users.ErrNotFound {
			return "", err
		}
		break
	}
	return externalID, nil
}

func (d *DB) findOrganizationByExternalID(externalID string) (*users.Organization, error) {
	for _, o := range d.organizations {
		if strings.ToLower(o.ExternalID) == strings.ToLower(externalID) {
			return o, nil
		}
	}
	return nil, users.ErrNotFound
}

var (
	// Values here needed to fake enforcement of length limit for org.name
	// With real DB, limit is enforced thanks to column of type varchar(100)

	// Constraint here needs to match length constraints enforced elsewhere, see ...
	// service-ui:client/src/common/constants.js:INSTANCE_NAME_MAX_LENGTH
	// service/users/db/migrations/015_limit_length_organizations.up.sql
	organizationMaxLength = 100

	errorOrgNameLengthConstraint = pq.Error{
		Severity: "ERROR",
		Code:     "22001",
		Message:  fmt.Sprintf("value too long for type character varying(%v)", organizationMaxLength),
		File:     "service/users/db/memory/organization.go",
		// Members below are what would be in real error generated by postgres
		// File:     "varchar.c",
		// Line:     "623",
		// Routine:  "varchar",
	}
)

// CreateOrganization creates a new organization owned by the user
func (d *DB) CreateOrganization(_ context.Context, ownerID, externalID, name, token string) (*users.Organization, error) {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	if _, err := d.findUserByID(ownerID); err != nil {
		return nil, err
	}
	if len(name) > organizationMaxLength {
		return nil, &errorOrgNameLengthConstraint
	}
	o := &users.Organization{
		ID:         fmt.Sprint(len(d.organizations)),
		ExternalID: externalID,
		Name:       name,
		CreatedAt:  time.Now().UTC(),
	}
	if err := o.Valid(); err != nil {
		return nil, err
	}
	if exists, err := d.organizationExists(o.ExternalID); err != nil {
		return nil, err
	} else if exists {
		return nil, users.ErrOrgExternalIDIsTaken
	}
	for exists := true; exists; {
		if token != "" {
			o.ProbeToken = token
		} else {
			if err := o.RegenerateProbeToken(); err != nil {
				return nil, err
			}
		}
		exists = false
		for _, org := range d.organizations {
			if org.ProbeToken == o.ProbeToken {
				exists = true
				break
			}
		}
		if token != "" && exists {
			return nil, users.ErrOrgTokenIsTaken
		}
	}
	d.organizations[o.ID] = o
	d.memberships[o.ID] = []string{ownerID}
	return o, nil
}

// FindOrganizationByProbeToken looks up the organization matching a given
// probe token.
func (d *DB) FindOrganizationByProbeToken(_ context.Context, probeToken string) (*users.Organization, error) {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	for _, o := range d.organizations {
		if o.ProbeToken == probeToken {
			return o, nil
		}
	}
	return nil, users.ErrNotFound
}

// FindOrganizationByID looks up the organization matching a given
// external id.
func (d *DB) FindOrganizationByID(_ context.Context, externalID string) (*users.Organization, error) {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	for _, o := range d.organizations {
		if o.ExternalID == externalID {
			return o, nil
		}
	}
	return nil, users.ErrNotFound
}

func changeOrg(d *DB, externalID string, toWrap func(*users.Organization) error) error {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	o, err := d.findOrganizationByExternalID(externalID)
	if err != nil {
		return err
	}

	return toWrap(o)
}

// RenameOrganization changes an organization's user-settable name
func (d *DB) RenameOrganization(_ context.Context, externalID, name string) error {
	if len(name) > organizationMaxLength {
		return &errorOrgNameLengthConstraint
	}
	if err := (&users.Organization{ExternalID: externalID, Name: name}).Valid(); err != nil {
		return err
	}

	return changeOrg(d, externalID, func(o *users.Organization) error {
		o.Name = name
		return nil
	})
}

// OrganizationExists just returns a simple bool checking if an organization
// exists
func (d *DB) OrganizationExists(_ context.Context, externalID string) (bool, error) {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	return d.organizationExists(externalID)
}

func (d *DB) organizationExists(externalID string) (bool, error) {
	if _, err := d.findOrganizationByExternalID(externalID); err == users.ErrNotFound {
		for _, deleted := range d.deletedOrganizations {
			if strings.ToLower(deleted.ExternalID) == strings.ToLower(externalID) {
				return true, nil
			}
		}
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

// GetOrganizationName gets the name of an organization from it's external ID.
func (d *DB) GetOrganizationName(_ context.Context, externalID string) (string, error) {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	o, err := d.findOrganizationByExternalID(externalID)
	if err != nil {
		return "", err
	}
	return o.Name, nil
}

// DeleteOrganization deletes an organization
func (d *DB) DeleteOrganization(_ context.Context, externalID string) error {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	o, err := d.findOrganizationByExternalID(externalID)
	if err == users.ErrNotFound {
		return nil
	}
	if err != nil {
		return err
	}
	d.deletedOrganizations[o.ID] = o
	delete(d.organizations, o.ID)
	delete(d.memberships, o.ID)
	return nil
}

// AddFeatureFlag adds a new feature flag to an organization.
func (d *DB) AddFeatureFlag(_ context.Context, externalID string, featureFlag string) error {
	return changeOrg(d, externalID, func(o *users.Organization) error {
		o.FeatureFlags = append(o.FeatureFlags, featureFlag)
		return nil
	})
}

// SetFeatureFlags sets all feature flags of an organization.
func (d *DB) SetFeatureFlags(_ context.Context, externalID string, featureFlags []string) error {
	return changeOrg(d, externalID, func(o *users.Organization) error {
		o.FeatureFlags = featureFlags
		return nil
	})
}

// SetOrganizationDenyUIFeatures sets the "deny UI features" flag on an organization
func (d *DB) SetOrganizationDenyUIFeatures(_ context.Context, externalID string, value bool) error {
	return changeOrg(d, externalID, func(org *users.Organization) error {
		org.DenyUIFeatures = value
		return nil
	})
}

// SetOrganizationDenyTokenAuth sets the "deny token auth" flag on an organization
func (d *DB) SetOrganizationDenyTokenAuth(_ context.Context, externalID string, value bool) error {
	return changeOrg(d, externalID, func(org *users.Organization) error {
		org.DenyTokenAuth = value
		return nil
	})
}

// SetOrganizationFirstSeenConnectedAt sets the first time an organisation has been connected
func (d *DB) SetOrganizationFirstSeenConnectedAt(_ context.Context, externalID string, value *time.Time) error {
	return changeOrg(d, externalID, func(org *users.Organization) error {
		org.FirstSeenConnectedAt = value
		return nil
	})
}

// SetOrganizationPlatformEnvironment sets the organisation platform and environment
func (d *DB) SetOrganizationPlatformEnvironment(_ context.Context, externalID, platform, environment string) error {
	return changeOrg(d, externalID, func(org *users.Organization) error {
		org.Platform = platform
		org.Environment = environment
		return nil
	})
}
