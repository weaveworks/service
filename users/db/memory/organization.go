package memory

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/lib/pq"

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
		if f.MatchesOrg(*org) {
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
	var err error
	for used := true; used && err == nil; {
		externalID = externalIDs.Generate()
		used, err = d.organizationExists(externalID, true)
	}
	return externalID, err
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
	now := time.Now().UTC()
	o := &users.Organization{
		ID:             fmt.Sprint(len(d.organizations)),
		ExternalID:     externalID,
		Name:           name,
		CreatedAt:      now,
		TrialExpiresAt: now.Add(users.TrialDuration),
	}
	if err := o.Valid(); err != nil {
		return nil, err
	}
	if exists, err := d.organizationExists(o.ExternalID, true); err != nil {
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

// FindOrganizationByGCPAccountID returns the organization with the given account ID.
func (d *DB) FindOrganizationByGCPAccountID(_ context.Context, accountID string) (*users.Organization, error) {
	for _, o := range d.organizations {
		if o.GCP != nil && o.GCP.AccountID == accountID {
			return o, nil
		}
	}
	return nil, users.ErrNotFound
}

// FindOrganizationByInternalID finds an org by its internal ID
func (d *DB) FindOrganizationByInternalID(ctx context.Context, internalID string) (*users.Organization, error) {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	for _, o := range d.organizations {
		if o.ID == internalID {
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

// UpdateOrganization changes an organization's user-settable name
func (d *DB) UpdateOrganization(_ context.Context, externalID string, update users.OrgWriteView) error {
	if update.Name != nil && len(*update.Name) > organizationMaxLength {
		return &errorOrgNameLengthConstraint
	}
	return changeOrg(d, externalID, func(o *users.Organization) error {
		if update.Name != nil {
			o.Name = *update.Name
		}
		if update.Platform != nil {
			o.Platform = *update.Platform
		}
		if update.Environment != nil {
			o.Environment = *update.Environment
		}
		if update.TrialExpiresAt != nil {
			o.TrialExpiresAt = *update.TrialExpiresAt
		}
		if update.TrialExpiredNotifiedAt != nil {
			o.TrialExpiredNotifiedAt = update.TrialExpiredNotifiedAt
		}
		if update.TrialPendingExpiryNotifiedAt != nil {
			o.TrialPendingExpiryNotifiedAt = update.TrialPendingExpiryNotifiedAt
		}

		return o.Valid()
	})
}

// OrganizationExists just returns a simple bool checking if an organization
// exists
func (d *DB) OrganizationExists(_ context.Context, externalID string) (bool, error) {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	return d.organizationExists(externalID, false)
}

// ExternalIDUsed returns true if the given `externalID` has ever been in use for
// an organization.
func (d *DB) ExternalIDUsed(_ context.Context, externalID string) (bool, error) {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	return d.organizationExists(externalID, true)
}

func (d *DB) organizationExists(externalID string, includeDeleted bool) (bool, error) {
	if _, err := d.findOrganizationByExternalID(externalID); err == users.ErrNotFound {
		if includeDeleted {
			for _, deleted := range d.deletedOrganizations {
				if strings.ToLower(deleted.ExternalID) == strings.ToLower(externalID) {
					return true, nil
				}
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

// SetOrganizationRefuseDataAccess sets the "deny UI features" flag on an organization
func (d *DB) SetOrganizationRefuseDataAccess(_ context.Context, externalID string, value bool) error {
	return changeOrg(d, externalID, func(org *users.Organization) error {
		org.RefuseDataAccess = value
		return nil
	})
}

// SetOrganizationRefuseDataUpload sets the "deny token auth" flag on an organization
func (d *DB) SetOrganizationRefuseDataUpload(_ context.Context, externalID string, value bool) error {
	return changeOrg(d, externalID, func(org *users.Organization) error {
		org.RefuseDataUpload = value
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

// SetOrganizationZuoraAccount sets the account number and time it was created at.
func (d *DB) SetOrganizationZuoraAccount(_ context.Context, externalID, number string, createdAt *time.Time) error {
	return changeOrg(d, externalID, func(org *users.Organization) error {
		org.ZuoraAccountNumber = number
		org.ZuoraAccountCreatedAt = createdAt
		return nil
	})
}

// CreateOrganizationWithGCP creates an organization as well as a GCP subscription, then links them together.
func (d *DB) CreateOrganizationWithGCP(ctx context.Context, ownerID, accountID, consumerID, subscriptionName, subscriptionLevel string) (*users.Organization, *users.GoogleCloudPlatform, error) {
	var org *users.Organization
	var gcp *users.GoogleCloudPlatform
	externalID, err := d.GenerateOrganizationExternalID(ctx)
	if err != nil {
		return nil, nil, err
	}
	name := users.DefaultOrganizationName(externalID)
	org, err = d.CreateOrganization(ctx, ownerID, externalID, name, "")
	if err != nil {
		return nil, nil, err
	}

	// Create and attach inactive GCP subscription to the organization
	gcp, err = d.createGCP(ctx, accountID, consumerID, subscriptionName, subscriptionLevel)
	if err != nil {
		return nil, nil, err
	}

	err = d.SetOrganizationGCP(ctx, externalID, accountID)
	if err != nil {
		return nil, nil, err
	}

	return org, gcp, nil
}

// FindGCP returns the Google Cloud Platform subscription for the given account.
func (d *DB) FindGCP(ctx context.Context, accountID string) (*users.GoogleCloudPlatform, error) {
	d.mtx.Lock()
	defer d.mtx.Unlock()

	return nil, nil
}

// UpdateGCP updates a Google Cloud Platform subscription.
func (d *DB) UpdateGCP(ctx context.Context, accountID, consumerID, subscriptionName, subscriptionLevel string, active bool) error {
	d.mtx.Lock()
	defer d.mtx.Unlock()

	gcp, exists := d.gcpSubscriptions[accountID]
	if !exists {
		return errors.New("Account not found")
	}
	gcp.AccountID = accountID
	gcp.ConsumerID = consumerID
	gcp.SubscriptionName = subscriptionName
	gcp.SubscriptionLevel = subscriptionLevel
	gcp.Active = active

	d.gcpSubscriptions[accountID] = gcp
	return nil
}

// SetOrganizationGCP attaches a Google Cloud Platform subscription to an organization.
// It also enables the billing feature flag and sets platform/env.
func (d *DB) SetOrganizationGCP(ctx context.Context, externalID, accountID string) error {
	d.mtx.Lock()
	defer d.mtx.Unlock()

	o, err := d.findOrganizationByExternalID(externalID)
	if err != nil {
		return err
	}
	if o.GCP != nil {
		return errors.New("Organization already has a GCP account")
	}

	o.GCP = d.gcpSubscriptions[accountID]

	// Hardcode platform/env here, that's what we expect the user to have.
	// It also skips the platform/env tab during the onboarding process.
	o.Platform = "kubernetes"
	o.Environment = "gke"

	// Enable billing otherwise we won't upload usage
	if !o.HasFeatureFlag(users.BillingFeatureFlag) {
		o.FeatureFlags = append(o.FeatureFlags, users.BillingFeatureFlag)
	}

	return nil
}

// CreateGCP creates a Google Cloud Platform account/subscription. It is initialized as inactive.
func (d *DB) createGCP(ctx context.Context, accountID, consumerID, subscriptionName, subscriptionLevel string) (*users.GoogleCloudPlatform, error) {
	d.mtx.Lock()
	defer d.mtx.Unlock()

	if _, exists := d.gcpSubscriptions[accountID]; exists {
		// If account is already known, Google either sent as a duplicate or we wrongfully called this method.
		return nil, errors.New("Account is already in use, reactivate subscription in the launcher")
	}
	gcp := &users.GoogleCloudPlatform{
		ID:                fmt.Sprint(len(d.gcpSubscriptions)),
		AccountID:         accountID,
		Active:            false,
		ConsumerID:        consumerID,
		SubscriptionName:  subscriptionName,
		SubscriptionLevel: subscriptionLevel,
	}
	d.gcpSubscriptions[accountID] = gcp
	return gcp, nil
}
