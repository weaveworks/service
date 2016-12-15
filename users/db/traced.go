package db

import (
	"encoding/json"

	"github.com/Sirupsen/logrus"
	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/login"
)

// traced adds logrus trace lines on each db call
type traced struct {
	d DB
}

func (t traced) trace(name string, args ...interface{}) {
	logrus.Debugf("%s: %#v", name, args)
}

func (t traced) CreateUser(email string) (u *users.User, err error) {
	defer func() { t.trace("CreateUser", email, u, err) }()
	return t.d.CreateUser(email)
}

func (t traced) FindUserByID(id string) (u *users.User, err error) {
	defer func() { t.trace("FindUserByID", id, u, err) }()
	return t.d.FindUserByID(id)
}

func (t traced) FindUserByEmail(email string) (u *users.User, err error) {
	defer func() { t.trace("FindUserByEmail", email, u, err) }()
	return t.d.FindUserByEmail(email)
}

func (t traced) FindUserByLogin(provider, id string) (u *users.User, err error) {
	defer func() { t.trace("FindUserByLogin", provider, id, u, err) }()
	return t.d.FindUserByLogin(provider, id)
}

func (t traced) FindUserByAPIToken(token string) (u *users.User, err error) {
	defer func() { t.trace("FindUserByAPIToken", token, u, err) }()
	return t.d.FindUserByAPIToken(token)
}

func (t traced) UserIsMemberOf(userID, orgExternalID string) (b bool, err error) {
	defer func() { t.trace("UserIsMemberOf", userID, orgExternalID, b, err) }()
	return t.d.UserIsMemberOf(userID, orgExternalID)
}

func (t traced) AddLoginToUser(userID, provider, id string, session json.RawMessage) (err error) {
	defer func() { t.trace("AddLoginToUser", userID, provider, id, session, err) }()
	return t.d.AddLoginToUser(userID, provider, id, session)
}

func (t traced) DetachLoginFromUser(userID, provider string) (err error) {
	defer func() { t.trace("DetachLoginFromUser", userID, provider, err) }()
	return t.d.DetachLoginFromUser(userID, provider)
}

func (t traced) CreateAPIToken(userID, description string) (token *users.APIToken, err error) {
	defer func() { t.trace("CreateAPIToken", userID, description, token, err) }()
	return t.d.CreateAPIToken(userID, description)
}

func (t traced) DeleteAPIToken(userID, token string) (err error) {
	defer func() { t.trace("DeleteAPIToken", userID, token, err) }()
	return t.d.DeleteAPIToken(userID, token)
}

func (t traced) InviteUser(email, orgExternalID string) (u *users.User, created bool, err error) {
	defer func() { t.trace("InviteUser", email, orgExternalID, u, created, err) }()
	return t.d.InviteUser(email, orgExternalID)
}

func (t traced) RemoveUserFromOrganization(orgExternalID, email string) (err error) {
	defer func() { t.trace("RemoveUserFromOrganization", orgExternalID, email, err) }()
	return t.d.RemoveUserFromOrganization(orgExternalID, email)
}

func (t traced) ListUsers() (us []*users.User, err error) {
	defer func() { t.trace("ListUsers", us, err) }()
	return t.d.ListUsers()
}

func (t traced) ListOrganizations() (os []*users.Organization, err error) {
	defer func() { t.trace("ListOrganizations", os, err) }()
	return t.d.ListOrganizations()
}

func (t traced) ListOrganizationUsers(orgExternalID string) (us []*users.User, err error) {
	defer func() { t.trace("ListOrganizationUsers", orgExternalID, us, err) }()
	return t.d.ListOrganizationUsers(orgExternalID)
}

func (t traced) ListOrganizationsForUserIDs(userIDs ...string) (os []*users.Organization, err error) {
	defer func() { t.trace("ListOrganizationsForUserIDs", userIDs, os, err) }()
	return t.d.ListOrganizationsForUserIDs(userIDs...)
}

func (t traced) ListLoginsForUserIDs(userIDs ...string) (ls []*login.Login, err error) {
	defer func() { t.trace("ListLoginsForUserIDs", userIDs, ls, err) }()
	return t.d.ListLoginsForUserIDs(userIDs...)
}

func (t traced) ListAPITokensForUserIDs(userIDs ...string) (ts []*users.APIToken, err error) {
	defer func() { t.trace("ListAPITokensForUserIDs", userIDs, ts, err) }()
	return t.d.ListAPITokensForUserIDs(userIDs...)
}

func (t traced) SetUserAdmin(id string, value bool) (err error) {
	defer func() { t.trace("SetUserAdmin", id, value, err) }()
	return t.d.SetUserAdmin(id, value)
}

func (t traced) SetUserToken(id, token string) (err error) {
	defer func() { t.trace("SetUserToken", id, token, err) }()
	return t.d.SetUserToken(id, token)
}

func (t traced) SetUserFirstLoginAt(id string) (err error) {
	defer func() { t.trace("SetUserFirstLoginAt", id, err) }()
	return t.d.SetUserFirstLoginAt(id)
}

func (t traced) GenerateOrganizationExternalID() (s string, err error) {
	defer func() { t.trace("GenerateOrganizationExternalID", s, err) }()
	return t.d.GenerateOrganizationExternalID()
}

func (t traced) CreateOrganization(ownerID, externalID, name string) (o *users.Organization, err error) {
	defer func() { t.trace("CreateOrganization", ownerID, externalID, name, o, err) }()
	return t.d.CreateOrganization(ownerID, externalID, name)
}

func (t traced) FindOrganizationByProbeToken(probeToken string) (o *users.Organization, err error) {
	defer func() { t.trace("FindOrganizationByProbeToken", probeToken, o, err) }()
	return t.d.FindOrganizationByProbeToken(probeToken)
}

func (t traced) FindOrganizationByID(externalID string) (o *users.Organization, err error) {
	defer func() { t.trace("FindOrganizationByID", externalID, o, err) }()
	return t.d.FindOrganizationByID(externalID)
}

func (t traced) RenameOrganization(externalID, name string) (err error) {
	defer func() { t.trace("RenameOrganization", externalID, name, err) }()
	return t.d.RenameOrganization(externalID, name)
}

func (t traced) OrganizationExists(externalID string) (b bool, err error) {
	defer func() { t.trace("OrganizationExists", externalID, b, err) }()
	return t.d.OrganizationExists(externalID)
}

func (t traced) GetOrganizationName(externalID string) (name string, err error) {
	defer func() { t.trace("GetOrganizationName", externalID, name, err) }()
	return t.d.GetOrganizationName(externalID)
}

func (t traced) DeleteOrganization(externalID string) (err error) {
	defer func() { t.trace("DeleteOrganization", externalID, err) }()
	return t.d.DeleteOrganization(externalID)
}

func (t traced) AddFeatureFlag(externalID string, featureFlag string) (err error) {
	defer func() { t.trace("AddFeatureFlag", externalID, featureFlag, err) }()
	return t.d.AddFeatureFlag(externalID, featureFlag)
}

func (t traced) SetFeatureFlags(externalID string, featureFlags []string) (err error) {
	defer func() { t.trace("SetFeatureFlags", externalID, featureFlags, err) }()
	return t.d.SetFeatureFlags(externalID, featureFlags)
}

func (t traced) ListMemberships() (ms []users.Membership, err error) {
	defer func() { t.trace("ListMemberships", err) }()
	return t.d.ListMemberships()
}

func (t traced) Close() (err error) {
	defer func() { t.trace("Close", err) }()
	return t.d.Close()
}
