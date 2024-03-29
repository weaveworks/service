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

// force interface compliance errors to occur here
var _ DB = &traced{}

func (t traced) trace(name string, args ...interface{}) {
	log.Debugf("%s: %#v", name, args)
}

func (t traced) CreateUser(ctx context.Context, email string, details *users.UserUpdate) (u *users.User, err error) {
	defer t.trace("CreateUser", email, u, err)
	return t.d.CreateUser(ctx, email, details)
}

func (t traced) UpdateUser(ctx context.Context, userID string, update *users.UserUpdate) (user *users.User, err error) {
	defer t.trace("UpdateUser", userID, update, user, err)
	return t.d.UpdateUser(ctx, userID, update)
}

func (t traced) DeleteUser(ctx context.Context, userID, actingID string) (err error) {
	defer t.trace("DeleteUser", userID, actingID, err)
	return t.d.DeleteUser(ctx, userID, actingID)
}

func (t traced) FindUserByID(ctx context.Context, id string) (u *users.User, err error) {
	defer t.trace("FindUserByID", id, u, err)
	return t.d.FindUserByID(ctx, id)
}

func (t traced) FindUserByEmail(ctx context.Context, email string) (u *users.User, err error) {
	defer t.trace("FindUserByEmail", email, u, err)
	return t.d.FindUserByEmail(ctx, email)
}

func (t traced) FindUserByLogin(ctx context.Context, provider, id string) (u *users.User, err error) {
	defer t.trace("FindUserByLogin", provider, id, u, err)
	return t.d.FindUserByLogin(ctx, provider, id)
}

func (t traced) UserIsMemberOf(ctx context.Context, userID, orgExternalID string) (b bool, err error) {
	defer t.trace("UserIsMemberOf", userID, orgExternalID, b, err)
	return t.d.UserIsMemberOf(ctx, userID, orgExternalID)
}

func (t traced) AddLoginToUser(ctx context.Context, userID, provider, id string, session json.RawMessage) (err error) {
	defer t.trace("AddLoginToUser", userID, provider, id, session, err)
	return t.d.AddLoginToUser(ctx, userID, provider, id, session)
}

func (t traced) GetLogin(ctx context.Context, provider, id string) (l *login.Login, err error) {
	defer t.trace("GetLogin", provider, id, l, err)
	return t.d.GetLogin(ctx, provider, id)
}

func (t traced) DetachLoginFromUser(ctx context.Context, userID, provider string) (err error) {
	defer t.trace("DetachLoginFromUser", userID, provider, err)
	return t.d.DetachLoginFromUser(ctx, userID, provider)
}

func (t traced) InviteUserToTeam(ctx context.Context, email, teamExternalID, roleID string) (u *users.User, created bool, err error) {
	defer t.trace("InviteUserToTeam", email, teamExternalID, roleID, u, created, err)
	return t.d.InviteUserToTeam(ctx, email, teamExternalID, roleID)
}

func (t traced) RemoveUserFromOrganization(ctx context.Context, orgExternalID, email string) (err error) {
	defer t.trace("RemoveUserFromOrganization", orgExternalID, email, err)
	return t.d.RemoveUserFromOrganization(ctx, orgExternalID, email)
}

func (t traced) RemoveUserFromTeam(ctx context.Context, userID, teamID string) (err error) {
	defer t.trace("RemoveUserFromTeam", userID, teamID, err)
	return t.d.RemoveUserFromTeam(ctx, userID, teamID)
}

func (t traced) ListUsers(ctx context.Context, f filter.User, page uint64) (us []*users.User, err error) {
	defer t.trace("ListUsers", page, us, err)
	return t.d.ListUsers(ctx, f, page)
}

func (t traced) ListOrganizations(ctx context.Context, f filter.Organization, page uint64) (os []*users.Organization, err error) {
	defer t.trace("ListOrganizations", page, os, err)
	return t.d.ListOrganizations(ctx, f, page)
}

func (t traced) ListAllOrganizations(ctx context.Context, f filter.Organization, orderBy string, page uint64) (os []*users.Organization, err error) {
	defer t.trace("ListAllOrganizations", orderBy, page, os, err)
	return t.d.ListAllOrganizations(ctx, f, orderBy, page)
}

func (t traced) ListOrganizationsInTeam(ctx context.Context, teamID string) (os []*users.Organization, err error) {
	defer t.trace("ListOrganizationsInTeam", teamID)
	return t.d.ListOrganizationsInTeam(ctx, teamID)
}

func (t traced) ListOrganizationUsers(ctx context.Context, orgExternalID string, includeDeletedOrgs, excludeNewUsers bool) (us []*users.User, err error) {
	defer t.trace("ListOrganizationUsers", orgExternalID, us, err)
	return t.d.ListOrganizationUsers(ctx, orgExternalID, includeDeletedOrgs, excludeNewUsers)
}

func (t traced) ListOrganizationsForUserIDs(ctx context.Context, userIDs ...string) (os []*users.Organization, err error) {
	defer t.trace("ListOrganizationsForUserIDs", userIDs, os, err)
	return t.d.ListOrganizationsForUserIDs(ctx, userIDs...)
}

func (t traced) ListAllOrganizationsForUserIDs(ctx context.Context, orderBy string, userIDs ...string) (os []*users.Organization, err error) {
	defer t.trace("ListAllOrganizationsForUserIDs", orderBy, userIDs, os, err)
	return t.d.ListAllOrganizationsForUserIDs(ctx, orderBy, userIDs...)
}

func (t traced) ListLoginsForUserIDs(ctx context.Context, userIDs ...string) (ls []*login.Login, err error) {
	defer t.trace("ListLoginsForUserIDs", userIDs, ls, err)
	return t.d.ListLoginsForUserIDs(ctx, userIDs...)
}

func (t traced) SetUserAdmin(ctx context.Context, id string, value bool) (err error) {
	defer t.trace("SetUserAdmin", id, value, err)
	return t.d.SetUserAdmin(ctx, id, value)
}

func (t traced) SetUserToken(ctx context.Context, id, token string) (err error) {
	defer t.trace("SetUserToken", id, token, err)
	return t.d.SetUserToken(ctx, id, token)
}

func (t traced) SetUserLastLoginAt(ctx context.Context, id string) (err error) {
	defer t.trace("SetUserLastLoginAt", id, err)
	return t.d.SetUserLastLoginAt(ctx, id)
}

func (t traced) GenerateOrganizationExternalID(ctx context.Context) (s string, err error) {
	defer t.trace("GenerateOrganizationExternalID", s, err)
	return t.d.GenerateOrganizationExternalID(ctx)
}

func (t traced) FindUncleanedOrgIDs(ctx context.Context) (ids []string, err error) {
	defer t.trace("FindUncleanedOrgIDs", ids, err)
	return t.d.FindUncleanedOrgIDs(ctx)
}

func (t traced) FindOrganizationByProbeToken(ctx context.Context, probeToken string) (o *users.Organization, err error) {
	defer t.trace("FindOrganizationByProbeToken", probeToken, o, err)
	return t.d.FindOrganizationByProbeToken(ctx, probeToken)
}

func (t traced) FindOrganizationByID(ctx context.Context, externalID string) (o *users.Organization, err error) {
	defer t.trace("FindOrganizationByID", externalID, o, err)
	return t.d.FindOrganizationByID(ctx, externalID)
}

func (t traced) FindOrganizationByGCPExternalAccountID(ctx context.Context, externalAccountID string) (o *users.Organization, err error) {
	defer t.trace("FindOrganizationByGCPExternalAccountID", externalAccountID, o, err)
	return t.d.FindOrganizationByGCPExternalAccountID(ctx, externalAccountID)
}

func (t traced) FindOrganizationByInternalID(ctx context.Context, internalID string) (o *users.Organization, err error) {
	defer t.trace("FindOrganizationByInternalID", internalID, o, err)
	return t.d.FindOrganizationByInternalID(ctx, internalID)
}

func (t traced) UpdateOrganization(ctx context.Context, externalID string, update users.OrgWriteView) (o *users.Organization, err error) {
	defer t.trace("UpdateOrganization", externalID, update, o, err)
	return t.d.UpdateOrganization(ctx, externalID, update)
}

func (t traced) MoveOrganizationToTeam(ctx context.Context, externalID, teamExternalID, teamName, userID string) (err error) {
	defer t.trace("MoveOrganizationToTeam", externalID, teamExternalID, teamName, userID, err)
	return t.d.MoveOrganizationToTeam(ctx, externalID, teamExternalID, teamName, userID)
}

func (t traced) OrganizationExists(ctx context.Context, externalID string) (b bool, err error) {
	defer t.trace("OrganizationExists", externalID, b, err)
	return t.d.OrganizationExists(ctx, externalID)
}

func (t traced) ExternalIDUsed(ctx context.Context, externalID string) (b bool, err error) {
	defer t.trace("ExternalIDUsed", externalID, b, err)
	return t.d.ExternalIDUsed(ctx, externalID)
}

func (t traced) GetOrganizationName(ctx context.Context, externalID string) (name string, err error) {
	defer t.trace("GetOrganizationName", externalID, name, err)
	return t.d.GetOrganizationName(ctx, externalID)
}

func (t traced) DeleteOrganization(ctx context.Context, externalID string, userID string) (err error) {
	defer t.trace("DeleteOrganization", externalID, err)
	return t.d.DeleteOrganization(ctx, externalID, userID)
}

func (t traced) AddFeatureFlag(ctx context.Context, externalID string, featureFlag string) (err error) {
	defer t.trace("AddFeatureFlag", externalID, featureFlag, err)
	return t.d.AddFeatureFlag(ctx, externalID, featureFlag)
}

func (t traced) SetFeatureFlags(ctx context.Context, externalID string, featureFlags []string) (err error) {
	defer t.trace("SetFeatureFlags", externalID, featureFlags, err)
	return t.d.SetFeatureFlags(ctx, externalID, featureFlags)
}

func (t traced) SetOrganizationCleanup(ctx context.Context, internalID string, value bool) (err error) {
	defer t.trace("SetOrganizationCleanup", internalID, err)
	return t.d.SetOrganizationCleanup(ctx, internalID, value)
}

func (t traced) SetOrganizationRefuseDataAccess(ctx context.Context, externalID string, value bool) (err error) {
	defer t.trace("SetOrganizationRefuseDataAccess", externalID, value, err)
	return t.d.SetOrganizationRefuseDataAccess(ctx, externalID, value)
}

func (t traced) SetOrganizationRefuseDataUpload(ctx context.Context, externalID string, value bool) (err error) {
	defer t.trace("SetOrganizationRefuseDataUpload", externalID, value, err)
	return t.d.SetOrganizationRefuseDataUpload(ctx, externalID, value)
}

func (t traced) SetOrganizationRefuseDataReason(ctx context.Context, externalID string, reason string) (err error) {
	defer t.trace("SetOrganizationRefuseDataReason", externalID, reason, err)
	return t.d.SetOrganizationRefuseDataReason(ctx, externalID, reason)
}

func (t traced) SetOrganizationFirstSeenConnectedAt(ctx context.Context, externalID string, value *time.Time) (err error) {
	defer t.trace("SetOrganizationFirstSeenConnectedAt", externalID, value, err)
	return t.d.SetOrganizationFirstSeenConnectedAt(ctx, externalID, value)
}

func (t traced) SetOrganizationFirstSeenFluxConnectedAt(ctx context.Context, externalID string, value *time.Time) (err error) {
	defer t.trace("SetOrganizationFirstSeenFluxConnectedAt", externalID, value, err)
	return t.d.SetOrganizationFirstSeenFluxConnectedAt(ctx, externalID, value)
}

func (t traced) SetOrganizationFirstSeenNetConnectedAt(ctx context.Context, externalID string, value *time.Time) (err error) {
	defer t.trace("SetOrganizationFirstSeenNetConnectedAt", externalID, value, err)
	return t.d.SetOrganizationFirstSeenNetConnectedAt(ctx, externalID, value)
}

func (t traced) SetOrganizationFirstSeenPromConnectedAt(ctx context.Context, externalID string, value *time.Time) (err error) {
	defer t.trace("SetOrganizationFirstSeenPromConnectedAt", externalID, value, err)
	return t.d.SetOrganizationFirstSeenPromConnectedAt(ctx, externalID, value)
}

func (t traced) SetOrganizationFirstSeenScopeConnectedAt(ctx context.Context, externalID string, value *time.Time) (err error) {
	defer t.trace("SetOrganizationFirstSeenScopeConnectedAt", externalID, value, err)
	return t.d.SetOrganizationFirstSeenScopeConnectedAt(ctx, externalID, value)
}

func (t traced) SetOrganizationZuoraAccount(ctx context.Context, externalID, number string, createdAt *time.Time) (err error) {
	defer t.trace("SetOrganizationZuoraAccount", externalID, number, createdAt, err)
	return t.d.SetOrganizationZuoraAccount(ctx, externalID, number, createdAt)
}

func (t traced) SetOrganizationPlatformVersion(ctx context.Context, externalID, platformVersion string) (err error) {
	defer t.trace("SetOrganizationPlatformVersion", externalID, platformVersion, err)
	return t.d.SetOrganizationPlatformVersion(ctx, externalID, platformVersion)
}

func (t traced) SetLastSentWeeklyReportAt(ctx context.Context, externalID string, sentAt *time.Time) (err error) {
	defer t.trace("SetLastSentWeeklyReportAt", externalID, sentAt, err)
	return t.d.SetLastSentWeeklyReportAt(ctx, externalID, sentAt)
}

func (t traced) CreateOrganizationWithGCP(ctx context.Context, ownerID, externalAccountID string, trialExpiresAt time.Time) (org *users.Organization, err error) {
	defer func() {
		t.trace("CreateOrganizationWithGCP", ownerID, externalAccountID, trialExpiresAt, org, err)
	}()
	return t.d.CreateOrganizationWithGCP(ctx, ownerID, externalAccountID, trialExpiresAt)
}

func (t traced) FindGCP(ctx context.Context, externalAccountID string) (gcp *users.GoogleCloudPlatform, err error) {
	defer t.trace("FindGCP", externalAccountID, gcp, err)
	return t.d.FindGCP(ctx, externalAccountID)
}

func (t traced) UpdateGCP(ctx context.Context, externalAccountID, consumerID, subscriptionName, subscriptionLevel, subscriptionStatus string) (err error) {
	defer func() {
		t.trace("UpdateGCP", externalAccountID, consumerID, subscriptionName, subscriptionLevel, subscriptionStatus, err)
	}()
	return t.d.UpdateGCP(ctx, externalAccountID, consumerID, subscriptionName, subscriptionLevel, subscriptionStatus)
}

func (t traced) SetOrganizationGCP(ctx context.Context, externalID, externalAccountID string) (err error) {
	defer t.trace("SetOrganizationGCP", externalID, externalAccountID, err)
	return t.d.SetOrganizationGCP(ctx, externalID, externalAccountID)
}

func (t traced) ListRoles(ctx context.Context) (r []*users.Role, err error) {
	defer t.trace("ListRoles", r, err)
	return t.d.ListRoles(ctx)
}

func (t traced) ListTeamsForUserID(ctx context.Context, userID string) (os []*users.Team, err error) {
	defer t.trace("ListTeamsForUserID", userID, os, err)
	return t.d.ListTeamsForUserID(ctx, userID)
}

func (t traced) ListTeamUsersWithRoles(ctx context.Context, teamID string) (os []*users.UserWithRole, err error) {
	defer t.trace("ListTeamUsersWithRoles", teamID, os, err)
	return t.d.ListTeamUsersWithRoles(ctx, teamID)
}

func (t traced) ListTeamUsers(ctx context.Context, teamID string) (os []*users.User, err error) {
	defer t.trace("ListTeamUsers", teamID, os, err)
	return t.d.ListTeamUsers(ctx, teamID)
}

func (t traced) ListTeams(ctx context.Context, page uint64) (ts []*users.Team, err error) {
	defer t.trace("ListTeams", ts, err)
	return t.d.ListTeams(ctx, page)
}

func (t traced) ListAllTeams(ctx context.Context, f filter.Team, orderBy string, page uint64) (ts []*users.Team, err error) {
	defer t.trace("ListAllTeams", orderBy, page, ts, err)
	return t.d.ListAllTeams(ctx, f, orderBy, page)
}

func (t traced) ListTeamMemberships(ctx context.Context) (ms []*users.TeamMembership, err error) {
	defer t.trace("ListTeamMemberships", ms, err)
	return t.d.ListTeamMemberships(ctx)
}

func (t traced) CreateTeam(ctx context.Context, name string) (ut *users.Team, err error) {
	defer t.trace("CreateTeam", name, ut, err)
	return t.d.CreateTeam(ctx, name)
}

func (t traced) AddUserToTeam(ctx context.Context, userID, teamID, roleID string) (err error) {
	defer t.trace("AddUserToTeam", userID, teamID, roleID, err)
	return t.d.AddUserToTeam(ctx, userID, teamID, roleID)
}

func (t traced) DeleteTeam(ctx context.Context, teamID string) (err error) {
	defer t.trace("DeleteTeam", teamID, err)
	return t.d.DeleteTeam(ctx, teamID)
}

func (t traced) ListPermissionsForRoleID(ctx context.Context, roleID string) (os []*users.Permission, err error) {
	defer t.trace("ListPermissionsForRoleID", roleID, os, err)
	return t.d.ListPermissionsForRoleID(ctx, roleID)
}

func (t traced) FindTeamByExternalID(ctx context.Context, externalID string) (team *users.Team, err error) {
	defer t.trace("FindTeamByExternalID", externalID, team, err)
	return t.d.FindTeamByExternalID(ctx, externalID)
}

func (t traced) FindTeamByInternalID(ctx context.Context, internalID string) (team *users.Team, err error) {
	defer t.trace("FindTeamByInternalID", internalID, team, err)
	return t.d.FindTeamByInternalID(ctx, internalID)
}

func (t traced) GetUserRoleInTeam(ctx context.Context, userID, teamID string) (r *users.Role, err error) {
	defer t.trace("GetUserRoleInTeam", userID, teamID, r, err)
	return t.d.GetUserRoleInTeam(ctx, userID, teamID)
}

func (t traced) UpdateUserRoleInTeam(ctx context.Context, userID, teamID, roleID string) (err error) {
	defer t.trace("UpdateUserRoleInTeam", userID, teamID, roleID, err)
	return t.d.UpdateUserRoleInTeam(ctx, userID, teamID, roleID)
}

func (t traced) CreateOrganizationWithTeam(ctx context.Context, ownerID, externalID, name, token, teamExternalID, teamName string, trialExpiresAt time.Time) (o *users.Organization, err error) {
	defer func() {
		t.trace("CreateOrganizationWithTeam", ownerID, externalID, name, token, teamExternalID, teamName, trialExpiresAt, o, err)
	}()
	return t.d.CreateOrganizationWithTeam(ctx, ownerID, externalID, name, token, teamExternalID, teamName, trialExpiresAt)
}

func (t traced) GetSummary(ctx context.Context) (entries []*users.SummaryEntry, err error) {
	defer t.trace("GetSummary", entries, err)
	return t.d.GetSummary(ctx)
}

func (t traced) ListOrganizationWebhooks(ctx context.Context, orgExternalID string) (ws []*users.Webhook, err error) {
	defer t.trace("ListOrganizationWebhooks", orgExternalID, ws, err)
	return t.d.ListOrganizationWebhooks(ctx, orgExternalID)
}

func (t traced) CreateOrganizationWebhook(ctx context.Context, orgExternalID, integrationType string) (w *users.Webhook, err error) {
	defer t.trace("CreateOrganizationWebhook", orgExternalID, integrationType, w, err)
	return t.d.CreateOrganizationWebhook(ctx, orgExternalID, integrationType)
}

func (t traced) DeleteOrganizationWebhook(ctx context.Context, orgExternalID, secretID string) (err error) {
	defer t.trace("DeleteOrganizationWebhook", orgExternalID, secretID, err)
	return t.d.DeleteOrganizationWebhook(ctx, orgExternalID, secretID)
}

func (t traced) FindOrganizationWebhookBySecretID(ctx context.Context, secretID string) (w *users.Webhook, err error) {
	defer t.trace("FindOrganizationWebhookBySecretID", secretID, w, err)
	return t.d.FindOrganizationWebhookBySecretID(ctx, secretID)
}

func (t traced) SetOrganizationWebhookFirstSeenAt(ctx context.Context, secretID string) (ti *time.Time, err error) {
	defer t.trace("SetOrganizationWebhookFirstSeenAt", secretID, ti, err)
	return t.d.SetOrganizationWebhookFirstSeenAt(ctx, secretID)
}

func (t traced) Close(ctx context.Context) (err error) {
	defer t.trace("Close", err)
	return t.d.Close(ctx)
}
