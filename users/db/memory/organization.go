package memory

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/lib/pq"

	"github.com/weaveworks/service/common/featureflag"
	timeutil "github.com/weaveworks/service/common/time"
	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/db/filter"
	"github.com/weaveworks/service/users/externalids"
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

	newTeams := map[string]string{}
	for teamID, roleID := range d.teamMemberships[u.ID] {
		if teamID != o.TeamID {
			newTeams[teamID] = roleID
		}
	}
	d.teamMemberships[u.ID] = newTeams

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

	teamIDs, _ := d.teamMemberships[userID]
	for teamID := range teamIDs {
		if teamID == o.TeamID {
			return true, nil
		}
	}
	return false, nil
}

// ListOrganizations lists organizations
func (d *DB) ListOrganizations(_ context.Context, f filter.Organization, page uint64) ([]*users.Organization, error) {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	var orgs []*users.Organization

	for _, org := range d.organizations {
		if f.MatchesOrg(*org) {
			orgs = append(orgs, org)
		}
	}
	sort.Sort(organizationsByCreatedAt(orgs))
	return orgs, nil
}

// ListAllOrganizations lists all organizations including deleted ones
// FIXME: orderBy is NOT implemented!
func (d *DB) ListAllOrganizations(_ context.Context, f filter.Organization, orderBy string, page uint64) ([]*users.Organization, error) {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	var orgs []*users.Organization

	for _, org := range d.organizations {
		if f.MatchesOrg(*org) {
			orgs = append(orgs, org)
		}
	}
	for _, org := range d.deletedOrganizations {
		if f.MatchesOrg(*org) {
			orgs = append(orgs, org)
		}
	}
	sort.Sort(organizationsByCreatedAt(orgs))
	return orgs, nil
}

// ListOrganizationsInTeam returns all organizations that are part of given team.
func (d *DB) ListOrganizationsInTeam(ctx context.Context, teamID string) ([]*users.Organization, error) {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	var orgs []*users.Organization

	for _, org := range d.organizations {
		if org.TeamID == teamID {
			orgs = append(orgs, org)
		}
	}
	sort.Sort(organizationsByCreatedAt(orgs))
	return orgs, nil
}

// ListOrganizationUsers lists all the users in an organization
func (d *DB) ListOrganizationUsers(ctx context.Context, orgExternalID string, includeDeletedOrgs, excludeNewUsers bool) ([]*users.User, error) {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	return d.listOrganizationUsers(ctx, orgExternalID, includeDeletedOrgs, excludeNewUsers)
}

// listOrganizationUsers lists all the users in an organization
// This is a lock-free version of the above, in order to be able to re-use the actual logic
// in other methods as otherwise, calling mtx.Lock() twice on the same goroutine deadlocks it.
func (d *DB) listOrganizationUsers(ctx context.Context, orgExternalID string, includeDeletedOrgs, excludeNewUsers bool) ([]*users.User, error) {
	o, err := d.findOrganizationByExternalID(orgExternalID)
	if err != nil {
		return nil, err
	}

	var us []*users.User
	if o.Deleted() && !includeDeletedOrgs {
		return us, nil
	}

	teamUsers, err := d.listTeamUsers(ctx, o.TeamID, excludeNewUsers)
	if err != nil {
		return nil, err
	}
	us = append(us, teamUsers...)

	sort.Sort(usersByCreatedAt(us))
	return us, nil
}

type organizationsByCreatedAt []*users.Organization

func (o organizationsByCreatedAt) Len() int           { return len(o) }
func (o organizationsByCreatedAt) Swap(i, j int)      { o[i], o[j] = o[j], o[i] }
func (o organizationsByCreatedAt) Less(i, j int) bool { return o[i].CreatedAt.After(o[j].CreatedAt) }

// ListOrganizationsForUserIDs lists the organizations these users belong to
func (d *DB) ListOrganizationsForUserIDs(_ context.Context, userIDs ...string) ([]*users.Organization, error) {
	return d.listOrganizationsForUserIDs(userIDs, false, "")
}

// ListAllOrganizationsForUserIDs lists the organizations these users
// belong to, including deleted ones.
func (d *DB) ListAllOrganizationsForUserIDs(_ context.Context, orderBy string, userIDs ...string) ([]*users.Organization, error) {
	return d.listOrganizationsForUserIDs(userIDs, true, orderBy)
}

// FIXME: orderBy is not implemented!
func (d *DB) listOrganizationsForUserIDs(userIDs []string, includeDeletedOrgs bool, orderBy string) ([]*users.Organization, error) {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	orgIDs := map[string]struct{}{}

	for _, userID := range userIDs {
		for teamID := range d.teamMemberships[userID] {
			for _, o := range d.organizations {
				if o.TeamID == teamID {
					orgIDs[o.ID] = struct{}{}
				}
			}
			if includeDeletedOrgs {
				for _, o := range d.deletedOrganizations {
					if o.TeamID == teamID {
						orgIDs[o.ID] = struct{}{}
					}
				}
			}
		}
	}

	var orgs []*users.Organization
	for orgID := range orgIDs {
		o, ok := d.organizations[orgID]
		if !ok && includeDeletedOrgs {
			o, ok = d.deletedOrganizations[orgID]
		}
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
		externalID = externalids.Generate()
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

	errorOrgNameLengthConstraint = &pq.Error{
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

// FindUncleanedOrgIDs looks up deleted but uncleaned organization IDs
func (d *DB) FindUncleanedOrgIDs(_ context.Context) ([]string, error) {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	var ids []string
	for _, org := range d.organizations {
		if !org.Cleanup && (org.Deleted() || org.RefuseDataUpload) {
			ids = append(ids, org.ID)
		}
	}
	return ids, nil
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

// FindOrganizationByGCPExternalAccountID returns the organization with the given account ID.
func (d *DB) FindOrganizationByGCPExternalAccountID(_ context.Context, externalAccountID string) (*users.Organization, error) {
	for _, o := range d.organizations {
		if o.GCP != nil && o.GCP.ExternalAccountID == externalAccountID {
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

// UpdateOrganization changes an organization's data.
func (d *DB) UpdateOrganization(_ context.Context, externalID string, update users.OrgWriteView) (*users.Organization, error) {
	if update.Name != nil && len(*update.Name) > organizationMaxLength {
		return nil, errorOrgNameLengthConstraint
	}
	var org *users.Organization
	err := changeOrg(d, externalID, func(o *users.Organization) error {
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
			o.TrialExpiredNotifiedAt = timeutil.ZeroTimeIsNil(update.TrialExpiredNotifiedAt)
		}
		if update.TrialPendingExpiryNotifiedAt != nil {
			o.TrialPendingExpiryNotifiedAt = timeutil.ZeroTimeIsNil(update.TrialPendingExpiryNotifiedAt)
		}
		org = o
		return o.Valid()
	})
	if err != nil {
		return nil, err
	}

	return org, nil
}

// MoveOrganizationToTeam updates the team of the organization. It does *not* check team permissions.
func (d *DB) MoveOrganizationToTeam(ctx context.Context, externalID, teamExternalID, teamName, userID string) error {
	var team *users.Team
	var err error

	if teamName != "" {
		if team, err = d.CreateTeamAsUser(ctx, teamName, userID); err != nil {
			return err
		}
	} else {
		if team, err = d.FindTeamByExternalID(ctx, teamExternalID); err != nil {
			return err
		}
	}

	return changeOrg(d, externalID, func(o *users.Organization) error {
		o.TeamID = team.ID
		o.TeamExternalID = team.ExternalID
		return nil
	})
}

// FindTeamByExternalID finds team by its external ID
func (d *DB) FindTeamByExternalID(ctx context.Context, externalID string) (*users.Team, error) {
	for _, t := range d.teams {
		if t.ExternalID == externalID {
			return t, nil
		}
	}
	return nil, users.ErrNotFound
}

// FindTeamByInternalID finds team by its internal ID
func (d *DB) FindTeamByInternalID(ctx context.Context, internalID string) (*users.Team, error) {
	for _, t := range d.teams {
		if t.ID == internalID {
			return t, nil
		}
	}
	return nil, users.ErrNotFound
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
func (d *DB) DeleteOrganization(_ context.Context, externalID, actingID string) error {
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
	return nil
}

// AddFeatureFlag adds a new feature flag to an organization.
func (d *DB) AddFeatureFlag(_ context.Context, externalID, featureFlag string) error {
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

// SetOrganizationCleanup sets cleanup for organization with internalID
func (d *DB) SetOrganizationCleanup(ctx context.Context, internalID string, value bool) error {
	org, err := d.FindOrganizationByInternalID(ctx, internalID)
	if err != nil {
		return err
	}
	return changeOrg(d, org.ExternalID, func(org *users.Organization) error {
		org.Cleanup = value
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

// SetOrganizationRefuseDataReason overwrites the default reason for refusal.
func (d *DB) SetOrganizationRefuseDataReason(ctx context.Context, externalID string, reason string) error {
	return changeOrg(d, externalID, func(org *users.Organization) error {
		org.RefuseDataReason = reason
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

// SetOrganizationFirstSeenFluxConnectedAt sets the first time an organisation flux agent has been connected
func (d *DB) SetOrganizationFirstSeenFluxConnectedAt(_ context.Context, externalID string, value *time.Time) error {
	return changeOrg(d, externalID, func(org *users.Organization) error {
		org.FirstSeenFluxConnectedAt = value
		return nil
	})
}

// SetOrganizationFirstSeenNetConnectedAt sets the first time an organisation weave net agent has been connected
func (d *DB) SetOrganizationFirstSeenNetConnectedAt(_ context.Context, externalID string, value *time.Time) error {
	return changeOrg(d, externalID, func(org *users.Organization) error {
		org.FirstSeenNetConnectedAt = value
		return nil
	})
}

// SetOrganizationFirstSeenPromConnectedAt sets the first time an organisation prometheus agent has been connected
func (d *DB) SetOrganizationFirstSeenPromConnectedAt(_ context.Context, externalID string, value *time.Time) error {
	return changeOrg(d, externalID, func(org *users.Organization) error {
		org.FirstSeenPromConnectedAt = value
		return nil
	})
}

// SetOrganizationFirstSeenScopeConnectedAt sets the first time an organisation scope agent has been connected
func (d *DB) SetOrganizationFirstSeenScopeConnectedAt(_ context.Context, externalID string, value *time.Time) error {
	return changeOrg(d, externalID, func(org *users.Organization) error {
		org.FirstSeenScopeConnectedAt = value
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

// SetOrganizationPlatformVersion sets the instance platform version.
func (d *DB) SetOrganizationPlatformVersion(_ context.Context, externalID, platformVersion string) error {
	return changeOrg(d, externalID, func(org *users.Organization) error {
		org.PlatformVersion = platformVersion
		return nil
	})
}

// SetLastSentWeeklyReportAt sets the last time weekly report email was sent for the instance
func (d *DB) SetLastSentWeeklyReportAt(_ context.Context, externalID string, sentAt *time.Time) error {
	return changeOrg(d, externalID, func(org *users.Organization) error {
		org.LastSentWeeklyReportAt = sentAt
		return nil
	})
}

// CreateOrganizationWithGCP creates an organization with an inactive GCP account attached to it.
func (d *DB) CreateOrganizationWithGCP(ctx context.Context, ownerID, externalAccountID string, trialExpiresAt time.Time) (*users.Organization, error) {
	var org *users.Organization
	var gcp *users.GoogleCloudPlatform
	externalID, err := d.GenerateOrganizationExternalID(ctx)
	if err != nil {
		return nil, err
	}
	name := users.DefaultOrganizationName(externalID)
	teamName := users.DefaultTeamName(externalID)
	org, err = d.CreateOrganizationWithTeam(ctx, ownerID, externalID, name, "", "", teamName, trialExpiresAt)
	if err != nil {
		return nil, err
	}

	// Create and attach inactive GCP subscription to the organization
	gcp, err = d.createGCP(ctx, externalAccountID)
	if err != nil {
		return nil, err
	}

	err = d.SetOrganizationGCP(ctx, externalID, externalAccountID)
	if err != nil {
		return nil, err
	}

	org.GCP = gcp
	return org, nil
}

// FindGCP returns the Google Cloud Platform subscription for the given account.
func (d *DB) FindGCP(ctx context.Context, externalAccountID string) (*users.GoogleCloudPlatform, error) {
	d.mtx.Lock()
	defer d.mtx.Unlock()

	gcp, exists := d.gcpAccounts[externalAccountID]
	if !exists {
		return nil, users.ErrNotFound
	}
	return gcp, nil
}

// UpdateGCP Update a Google Cloud Platform entry. This marks the account as activated.
func (d *DB) UpdateGCP(ctx context.Context, externalAccountID, consumerID, subscriptionName, subscriptionLevel, subscriptionStatus string) error {
	d.mtx.Lock()
	defer d.mtx.Unlock()

	gcp, exists := d.gcpAccounts[externalAccountID]
	if !exists {
		return users.ErrNotFound
	}
	gcp.ExternalAccountID = externalAccountID
	gcp.ConsumerID = consumerID
	gcp.SubscriptionName = subscriptionName
	gcp.SubscriptionLevel = subscriptionLevel
	gcp.SubscriptionStatus = subscriptionStatus
	gcp.Activated = true

	d.gcpAccounts[externalAccountID] = gcp
	return nil
}

// SetOrganizationGCP attaches a Google Cloud Platform subscription to an organization.
// It also enables the billing feature flag and sets platform/env.
func (d *DB) SetOrganizationGCP(ctx context.Context, externalID, externalAccountID string) error {
	d.mtx.Lock()
	defer d.mtx.Unlock()

	o, err := d.findOrganizationByExternalID(externalID)
	if err != nil {
		return err
	}
	if o.GCP != nil {
		return errors.New("organization already has a GCP account")
	}

	o.GCP = d.gcpAccounts[externalAccountID]

	// Hardcode platform/env here, that's what we expect the user to have.
	// It also skips the platform/env tab during the onboarding process.
	o.Platform = "kubernetes"
	o.Environment = "gke"
	// No trial for GCP instances
	o.TrialExpiresAt = time.Now()

	// Enable billing otherwise we won't upload usage
	if !o.HasFeatureFlag(featureflag.Billing) {
		o.FeatureFlags = append(o.FeatureFlags, featureflag.Billing)
	}

	return nil
}

// CreateOrganizationWithTeam creates a new organization owned by the user
// If teamExternalID is not empty, the organizations is assigned to that team, if it exists.
// If teamName is not empty, the organizations is assigned to that team. it is created if it does not exists.
// One, and only one, of teamExternalID, teamName must be provided.
func (d *DB) CreateOrganizationWithTeam(ctx context.Context, ownerID, externalID, name, token, teamExternalID, teamName string, trialExpiresAt time.Time) (*users.Organization, error) {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	if _, err := d.findUserByID(ownerID); err != nil {
		return nil, err
	}
	if len(name) > organizationMaxLength {
		return nil, errorOrgNameLengthConstraint
	}
	now := time.Now().UTC()

	var team *users.Team
	var err error
	if teamExternalID != "" {
		team, err = d.getTeamUserIsPartOf(ctx, ownerID, teamExternalID)
	} else if teamName != "" {
		team, err = d.ensureUserIsPartOfTeamByName(ctx, ownerID, teamName)
	}
	if err != nil {
		return nil, err
	}

	o := &users.Organization{
		ID:             fmt.Sprint(len(d.organizations)),
		ExternalID:     externalID,
		Name:           name,
		CreatedAt:      now,
		TrialExpiresAt: trialExpiresAt,
		TeamID:         team.ID,
		TeamExternalID: team.ExternalID,
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
	return o, nil
}

// CreateGCP creates a Google Cloud Platform account/subscription. It is initialized as inactive.
func (d *DB) createGCP(ctx context.Context, externalAccountID string) (*users.GoogleCloudPlatform, error) {
	d.mtx.Lock()
	defer d.mtx.Unlock()

	if _, exists := d.gcpAccounts[externalAccountID]; exists {
		// If account is already known, Google either sent as a duplicate or we wrongfully called this method.
		return nil, errors.New("account is already in use, reactivate subscription in the launcher")
	}
	gcp := &users.GoogleCloudPlatform{
		ID:                fmt.Sprint(len(d.gcpAccounts)),
		ExternalAccountID: externalAccountID,
		Activated:         false,
	}
	d.gcpAccounts[externalAccountID] = gcp
	return gcp, nil
}

// GetSummary exports a summary of the DB.
func (d *DB) GetSummary(ctx context.Context) ([]*users.SummaryEntry, error) {
	d.mtx.Lock()
	defer d.mtx.Unlock()

	var entries []*users.SummaryEntry
	for _, org := range d.organizations {
		team := d.teams[org.TeamID]
		orgUsers, err := d.listOrganizationUsers(ctx, org.ExternalID, false, false)
		if err != nil {
			return nil, err
		}
		entries = append(entries, users.NewSummaryEntry(org, team, orgUsers))
	}
	sort.Sort(users.SummaryEntriesByCreatedAt(entries))
	return entries, nil
}
