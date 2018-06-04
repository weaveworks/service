package db

import (
	"context"
	"encoding/json"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/db/filter"
	"github.com/weaveworks/service/users/login"
)

// traced adds logrus trace lines on each db call
type traced struct {
	d DB
}

func (t traced) trace(name string, args ...interface{}) {
	log.Debugf("%s: %#v", name, args)
}

func (t traced) CreateUser(ctx context.Context, email string) (u *users.User, err error) {
	defer func() { t.trace("CreateUser", email, u, err) }()
	return t.d.CreateUser(ctx, email)
}

func (t traced) DeleteUser(ctx context.Context, userID string) (err error) {
	defer func() { t.trace("DeleteUser", userID, err) }()
	return t.d.DeleteUser(ctx, userID)
}

func (t traced) FindUserByID(ctx context.Context, id string) (u *users.User, err error) {
	defer func() { t.trace("FindUserByID", id, u, err) }()
	return t.d.FindUserByID(ctx, id)
}

func (t traced) FindUserByEmail(ctx context.Context, email string) (u *users.User, err error) {
	defer func() { t.trace("FindUserByEmail", email, u, err) }()
	return t.d.FindUserByEmail(ctx, email)
}

func (t traced) FindUserByLogin(ctx context.Context, provider, id string) (u *users.User, err error) {
	defer func() { t.trace("FindUserByLogin", provider, id, u, err) }()
	return t.d.FindUserByLogin(ctx, provider, id)
}

func (t traced) UserIsMemberOf(ctx context.Context, userID, orgExternalID string) (b bool, err error) {
	defer func() { t.trace("UserIsMemberOf", userID, orgExternalID, b, err) }()
	return t.d.UserIsMemberOf(ctx, userID, orgExternalID)
}

func (t traced) AddLoginToUser(ctx context.Context, userID, provider, id string, session json.RawMessage) (err error) {
	defer func() { t.trace("AddLoginToUser", userID, provider, id, session, err) }()
	return t.d.AddLoginToUser(ctx, userID, provider, id, session)
}

func (t traced) DetachLoginFromUser(ctx context.Context, userID, provider string) (err error) {
	defer func() { t.trace("DetachLoginFromUser", userID, provider, err) }()
	return t.d.DetachLoginFromUser(ctx, userID, provider)
}

func (t traced) InviteUser(ctx context.Context, email, orgExternalID string) (u *users.User, created bool, err error) {
	defer func() { t.trace("InviteUser", email, orgExternalID, u, created, err) }()
	return t.d.InviteUser(ctx, email, orgExternalID)
}

func (t traced) RemoveUserFromOrganization(ctx context.Context, orgExternalID, email string) (err error) {
	defer func() { t.trace("RemoveUserFromOrganization", orgExternalID, email, err) }()
	return t.d.RemoveUserFromOrganization(ctx, orgExternalID, email)
}

func (t traced) ListUsers(ctx context.Context, f filter.User, page uint64) (us []*users.User, err error) {
	defer func() { t.trace("ListUsers", page, us, err) }()
	return t.d.ListUsers(ctx, f, page)
}

func (t traced) ListOrganizations(ctx context.Context, f filter.Organization, page uint64) (os []*users.Organization, err error) {
	defer func() { t.trace("ListOrganizations", page, os, err) }()
	return t.d.ListOrganizations(ctx, f, page)
}

func (t traced) ListAllOrganizations(ctx context.Context, f filter.Organization, page uint64) (os []*users.Organization, err error) {
	defer func() { t.trace("ListAllOrganizations", page, os, err) }()
	return t.d.ListAllOrganizations(ctx, f, page)
}

func (t traced) ListOrganizationUsers(ctx context.Context, orgExternalID string) (us []*users.User, err error) {
	defer func() { t.trace("ListOrganizationUsers", orgExternalID, us, err) }()
	return t.d.ListOrganizationUsers(ctx, orgExternalID)
}

func (t traced) ListOrganizationsForUserIDs(ctx context.Context, userIDs ...string) (os []*users.Organization, err error) {
	defer func() { t.trace("ListOrganizationsForUserIDs", userIDs, os, err) }()
	return t.d.ListOrganizationsForUserIDs(ctx, userIDs...)
}

func (t traced) ListLoginsForUserIDs(ctx context.Context, userIDs ...string) (ls []*login.Login, err error) {
	defer func() { t.trace("ListLoginsForUserIDs", userIDs, ls, err) }()
	return t.d.ListLoginsForUserIDs(ctx, userIDs...)
}

func (t traced) SetUserAdmin(ctx context.Context, id string, value bool) (err error) {
	defer func() { t.trace("SetUserAdmin", id, value, err) }()
	return t.d.SetUserAdmin(ctx, id, value)
}

func (t traced) SetUserToken(ctx context.Context, id, token string) (err error) {
	defer func() { t.trace("SetUserToken", id, token, err) }()
	return t.d.SetUserToken(ctx, id, token)
}

func (t traced) SetUserLastLoginAt(ctx context.Context, id string) (err error) {
	defer func() { t.trace("SetUserLastLoginAt", id, err) }()
	return t.d.SetUserLastLoginAt(ctx, id)
}

func (t traced) GenerateOrganizationExternalID(ctx context.Context) (s string, err error) {
	defer func() { t.trace("GenerateOrganizationExternalID", s, err) }()
	return t.d.GenerateOrganizationExternalID(ctx)
}

func (t traced) CreateOrganization(ctx context.Context, ownerID, externalID, name, token, teamID string, trialExpiresAt time.Time) (o *users.Organization, err error) {
	defer func() {
		t.trace("CreateOrganization", ownerID, externalID, name, token, teamID, trialExpiresAt, o, err)
	}()
	return t.d.CreateOrganization(ctx, ownerID, externalID, name, token, teamID, trialExpiresAt)
}

func (t traced) FindUncleanedOrgIDs(ctx context.Context) (ids []string, err error) {
	defer func() { t.trace("FindUncleanedOrgIDs", ids, err) }()
	return t.d.FindUncleanedOrgIDs(ctx)
}

func (t traced) FindOrganizationByProbeToken(ctx context.Context, probeToken string) (o *users.Organization, err error) {
	defer func() { t.trace("FindOrganizationByProbeToken", probeToken, o, err) }()
	return t.d.FindOrganizationByProbeToken(ctx, probeToken)
}

func (t traced) FindOrganizationByID(ctx context.Context, externalID string) (o *users.Organization, err error) {
	defer func() { t.trace("FindOrganizationByID", externalID, o, err) }()
	return t.d.FindOrganizationByID(ctx, externalID)
}

func (t traced) FindOrganizationByGCPExternalAccountID(ctx context.Context, externalAccountID string) (o *users.Organization, err error) {
	defer func() { t.trace("FindOrganizationByGCPExternalAccountID", externalAccountID, o, err) }()
	return t.d.FindOrganizationByGCPExternalAccountID(ctx, externalAccountID)
}

func (t traced) FindOrganizationByInternalID(ctx context.Context, internalID string) (o *users.Organization, err error) {
	defer func() { t.trace("FindOrganizationByInternalID", internalID, o, err) }()
	return t.d.FindOrganizationByInternalID(ctx, internalID)
}

func (t traced) UpdateOrganization(ctx context.Context, externalID string, update users.OrgWriteView) (o *users.Organization, err error) {
	defer func() { t.trace("UpdateOrganization", externalID, update, o, err) }()
	return t.d.UpdateOrganization(ctx, externalID, update)
}

func (t traced) MoveOrganizationToTeam(ctx context.Context, externalID, teamExternalID, teamName, userID string) (err error) {
	defer func() { t.trace("MoveOrganizationToTeam", externalID, teamExternalID, teamName, userID, err) }()
	return t.d.MoveOrganizationToTeam(ctx, externalID, teamExternalID, teamName, userID)
}

func (t traced) OrganizationExists(ctx context.Context, externalID string) (b bool, err error) {
	defer func() { t.trace("OrganizationExists", externalID, b, err) }()
	return t.d.OrganizationExists(ctx, externalID)
}

func (t traced) ExternalIDUsed(ctx context.Context, externalID string) (b bool, err error) {
	defer func() { t.trace("ExternalIDUsed", externalID, b, err) }()
	return t.d.ExternalIDUsed(ctx, externalID)
}

func (t traced) GetOrganizationName(ctx context.Context, externalID string) (name string, err error) {
	defer func() { t.trace("GetOrganizationName", externalID, name, err) }()
	return t.d.GetOrganizationName(ctx, externalID)
}

func (t traced) DeleteOrganization(ctx context.Context, externalID string) (err error) {
	defer func() { t.trace("DeleteOrganization", externalID, err) }()
	return t.d.DeleteOrganization(ctx, externalID)
}

func (t traced) AddFeatureFlag(ctx context.Context, externalID string, featureFlag string) (err error) {
	defer func() { t.trace("AddFeatureFlag", externalID, featureFlag, err) }()
	return t.d.AddFeatureFlag(ctx, externalID, featureFlag)
}

func (t traced) SetFeatureFlags(ctx context.Context, externalID string, featureFlags []string) (err error) {
	defer func() { t.trace("SetFeatureFlags", externalID, featureFlags, err) }()
	return t.d.SetFeatureFlags(ctx, externalID, featureFlags)
}

func (t traced) SetOrganizationCleanup(ctx context.Context, internalID string, value bool) (err error) {
	defer func() { t.trace("SetOrganizationCleanup", internalID, err) }()
	return t.d.SetOrganizationCleanup(ctx, internalID, value)
}

func (t traced) SetOrganizationRefuseDataAccess(ctx context.Context, externalID string, value bool) (err error) {
	defer func() { t.trace("SetOrganizationRefuseDataAccess", externalID, value, err) }()
	return t.d.SetOrganizationRefuseDataAccess(ctx, externalID, value)
}

func (t traced) SetOrganizationRefuseDataUpload(ctx context.Context, externalID string, value bool) (err error) {
	defer func() { t.trace("SetOrganizationRefuseDataUpload", externalID, value, err) }()
	return t.d.SetOrganizationRefuseDataUpload(ctx, externalID, value)
}

func (t traced) SetOrganizationFirstSeenConnectedAt(ctx context.Context, externalID string, value *time.Time) (err error) {
	defer func() { t.trace("SetOrganizationFirstSeenConnectedAt", externalID, value, err) }()
	return t.d.SetOrganizationFirstSeenConnectedAt(ctx, externalID, value)
}

func (t traced) SetOrganizationFirstSeenFluxConnectedAt(ctx context.Context, externalID string, value *time.Time) (err error) {
	defer func() { t.trace("SetOrganizationFirstSeenFluxConnectedAt", externalID, value, err) }()
	return t.d.SetOrganizationFirstSeenFluxConnectedAt(ctx, externalID, value)
}

func (t traced) SetOrganizationFirstSeenNetConnectedAt(ctx context.Context, externalID string, value *time.Time) (err error) {
	defer func() { t.trace("SetOrganizationFirstSeenNetConnectedAt", externalID, value, err) }()
	return t.d.SetOrganizationFirstSeenNetConnectedAt(ctx, externalID, value)
}

func (t traced) SetOrganizationFirstSeenPromConnectedAt(ctx context.Context, externalID string, value *time.Time) (err error) {
	defer func() { t.trace("SetOrganizationFirstSeenPromConnectedAt", externalID, value, err) }()
	return t.d.SetOrganizationFirstSeenPromConnectedAt(ctx, externalID, value)
}

func (t traced) SetOrganizationFirstSeenScopeConnectedAt(ctx context.Context, externalID string, value *time.Time) (err error) {
	defer func() { t.trace("SetOrganizationFirstSeenScopeConnectedAt", externalID, value, err) }()
	return t.d.SetOrganizationFirstSeenScopeConnectedAt(ctx, externalID, value)
}

func (t traced) SetOrganizationZuoraAccount(ctx context.Context, externalID, number string, createdAt *time.Time) (err error) {
	defer func() { t.trace("SetOrganizationZuoraAccount", externalID, number, createdAt, err) }()
	return t.d.SetOrganizationZuoraAccount(ctx, externalID, number, createdAt)
}

func (t traced) CreateOrganizationWithGCP(ctx context.Context, ownerID, externalAccountID string, trialExpiresAt time.Time) (org *users.Organization, err error) {
	defer func() {
		t.trace("CreateOrganizationWithGCP", ownerID, externalAccountID, trialExpiresAt, org, err)
	}()
	return t.d.CreateOrganizationWithGCP(ctx, ownerID, externalAccountID, trialExpiresAt)
}

func (t traced) FindGCP(ctx context.Context, externalAccountID string) (gcp *users.GoogleCloudPlatform, err error) {
	defer func() { t.trace("FindGCP", externalAccountID, gcp, err) }()
	return t.d.FindGCP(ctx, externalAccountID)
}

func (t traced) UpdateGCP(ctx context.Context, externalAccountID, consumerID, subscriptionName, subscriptionLevel, subscriptionStatus string) (err error) {
	defer func() {
		t.trace("UpdateGCP", externalAccountID, consumerID, subscriptionName, subscriptionLevel, subscriptionStatus, err)
	}()
	return t.d.UpdateGCP(ctx, externalAccountID, consumerID, subscriptionName, subscriptionLevel, subscriptionStatus)
}

func (t traced) SetOrganizationGCP(ctx context.Context, externalID, externalAccountID string) (err error) {
	defer func() { t.trace("SetOrganizationGCP", externalID, externalAccountID, err) }()
	return t.d.SetOrganizationGCP(ctx, externalID, externalAccountID)
}

func (t traced) ListMemberships(ctx context.Context) (ms []users.Membership, err error) {
	defer func() { t.trace("ListMemberships", err) }()
	return t.d.ListMemberships(ctx)
}

func (t traced) ListTeamsForUserID(ctx context.Context, userID string) (os []*users.Team, err error) {
	defer func() { t.trace("ListTeamsForUserID", userID, os, err) }()
	return t.d.ListTeamsForUserID(ctx, userID)
}

func (t traced) ListTeamUsers(ctx context.Context, teamID string) (os []*users.User, err error) {
	defer func() { t.trace("ListTeamUsers", teamID, os, err) }()
	return t.d.ListTeamUsers(ctx, teamID)
}

func (t traced) CreateTeam(ctx context.Context, name string) (ut *users.Team, err error) {
	defer func() { t.trace("CreateTeam", name, ut, err) }()
	return t.d.CreateTeam(ctx, name)
}

func (t traced) AddUserToTeam(ctx context.Context, userID, teamID string) (err error) {
	defer func() { t.trace("AddUserToTeam", userID, teamID, err) }()
	return t.d.AddUserToTeam(ctx, userID, teamID)
}

func (t traced) CreateOrganizationWithTeam(ctx context.Context, ownerID, externalID, name, token, teamExternalID, teamName string, trialExpiresAt time.Time) (o *users.Organization, err error) {
	defer func() {
		t.trace("CreateOrganizationWithTeam", ownerID, externalID, name, token, teamExternalID, teamName, trialExpiresAt, o, err)
	}()
	return t.d.CreateOrganizationWithTeam(ctx, ownerID, externalID, name, token, teamExternalID, teamName, trialExpiresAt)
}

func (t traced) GetSummary(ctx context.Context) (entries []*users.SummaryEntry, err error) {
	defer func() { t.trace("GetSummary", entries, err) }()
	return t.d.GetSummary(ctx)
}

func (t traced) Close(ctx context.Context) (err error) {
	defer func() { t.trace("Close", err) }()
	return t.d.Close(ctx)
}
