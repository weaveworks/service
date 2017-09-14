package db

import (
	"encoding/json"
	"time"

	log "github.com/Sirupsen/logrus"
	"golang.org/x/net/context"

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

func (t traced) ListUsers(ctx context.Context, f filter.UserFilter) (us []*users.User, err error) {
	defer func() { t.trace("ListUsers", us, err) }()
	return t.d.ListUsers(ctx, f)
}

func (t traced) ListOrganizations(ctx context.Context, f filter.Organization) (os []*users.Organization, err error) {
	defer func() { t.trace("ListOrganizations", os, err) }()
	return t.d.ListOrganizations(ctx, f)
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

func (t traced) SetUserFirstLoginAt(ctx context.Context, id string) (err error) {
	defer func() { t.trace("SetUserFirstLoginAt", id, err) }()
	return t.d.SetUserFirstLoginAt(ctx, id)
}

func (t traced) GenerateOrganizationExternalID(ctx context.Context) (s string, err error) {
	defer func() { t.trace("GenerateOrganizationExternalID", s, err) }()
	return t.d.GenerateOrganizationExternalID(ctx)
}

func (t traced) CreateOrganization(ctx context.Context, ownerID, externalID, name, token string) (o *users.Organization, err error) {
	defer func() { t.trace("CreateOrganization", ownerID, externalID, name, token, o, err) }()
	return t.d.CreateOrganization(ctx, ownerID, externalID, name, token)
}

func (t traced) FindOrganizationByProbeToken(ctx context.Context, probeToken string) (o *users.Organization, err error) {
	defer func() { t.trace("FindOrganizationByProbeToken", probeToken, o, err) }()
	return t.d.FindOrganizationByProbeToken(ctx, probeToken)
}

func (t traced) FindOrganizationByID(ctx context.Context, externalID string) (o *users.Organization, err error) {
	defer func() { t.trace("FindOrganizationByID", externalID, o, err) }()
	return t.d.FindOrganizationByID(ctx, externalID)
}

func (t traced) UpdateOrganization(ctx context.Context, externalID string, update users.OrgWriteView) (err error) {
	defer func() { t.trace("UpdateOrganization", externalID, update, err) }()
	return t.d.UpdateOrganization(ctx, externalID, update)
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

func (t traced) SetOrganizationDenyUIFeatures(ctx context.Context, externalID string, value bool) (err error) {
	defer func() { t.trace("SetOrganizationDenyUIFeatures", externalID, value, err) }()
	return t.d.SetOrganizationDenyUIFeatures(ctx, externalID, value)
}

func (t traced) SetOrganizationDenyTokenAuth(ctx context.Context, externalID string, value bool) (err error) {
	defer func() { t.trace("SetOrganizationDenyTokenAuth", externalID, value, err) }()
	return t.d.SetOrganizationDenyTokenAuth(ctx, externalID, value)
}

func (t traced) SetOrganizationFirstSeenConnectedAt(ctx context.Context, externalID string, value *time.Time) (err error) {
	defer func() { t.trace("SetOrganizationFirstSeenConnectedAt", externalID, value, err) }()
	return t.d.SetOrganizationFirstSeenConnectedAt(ctx, externalID, value)
}

func (t traced) SetOrganizationZuoraAccount(ctx context.Context, externalID, number string, createdAt *time.Time) (err error) {
	defer func() { t.trace("SetOrganizationZuoraAccount", externalID, number, createdAt, err) }()
	return t.d.SetOrganizationZuoraAccount(ctx, externalID, number, createdAt)
}

func (t traced) ListMemberships(ctx context.Context) (ms []users.Membership, err error) {
	defer func() { t.trace("ListMemberships", err) }()
	return t.d.ListMemberships(ctx)
}

func (t traced) Close(ctx context.Context) (err error) {
	defer func() { t.trace("Close", err) }()
	return t.d.Close(ctx)
}
