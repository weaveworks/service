package db

import (
	"encoding/json"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/weaveworks/scope/common/instrument"

	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/login"
)

// timed adds prometheus timings to another database implementation
type timed struct {
	d        DB
	Duration *prometheus.HistogramVec
}

func (t timed) errorCode(err error) string {
	switch err {
	case nil:
		return "200"
	case users.ErrNotFound:
		return "404"
	case users.ErrEmailIsTaken:
		return "400"
	case users.ErrInvalidAuthenticationData:
		return "401"
	default:
		return "500"
	}
}

func (t timed) timeRequest(method string, f func() error) error {
	return instrument.TimeRequestHistogramStatus(method, t.Duration, t.errorCode, f)
}

func (t timed) CreateUser(email string) (u *users.User, err error) {
	t.timeRequest("CreateUser", func() error {
		u, err = t.d.CreateUser(email)
		return err
	})
	return
}

func (t timed) FindUserByID(id string) (u *users.User, err error) {
	t.timeRequest("FindUserByID", func() error {
		u, err = t.d.FindUserByID(id)
		return err
	})
	return
}

func (t timed) FindUserByEmail(email string) (u *users.User, err error) {
	t.timeRequest("FindUserByEmail", func() error {
		u, err = t.d.FindUserByEmail(email)
		return err
	})
	return
}

func (t timed) FindUserByLogin(provider, id string) (u *users.User, err error) {
	t.timeRequest("FindUserByLogin", func() error {
		u, err = t.d.FindUserByLogin(provider, id)
		return err
	})
	return
}

func (t timed) FindUserByAPIToken(token string) (u *users.User, err error) {
	t.timeRequest("FindUserByAPIToken", func() error {
		u, err = t.d.FindUserByAPIToken(token)
		return err
	})
	return
}

func (t timed) UserIsMemberOf(userID, orgExternalID string) (b bool, err error) {
	t.timeRequest("UserIsMemberOf", func() error {
		b, err = t.d.UserIsMemberOf(userID, orgExternalID)
		return err
	})
	return
}

func (t timed) AddLoginToUser(userID, provider, id string, session json.RawMessage) error {
	return t.timeRequest("AddLoginToUser", func() error {
		return t.d.AddLoginToUser(userID, provider, id, session)
	})
}

func (t timed) DetachLoginFromUser(userID, provider string) error {
	return t.timeRequest("DetachLoginFromUser", func() error {
		return t.d.DetachLoginFromUser(userID, provider)
	})
}

func (t timed) CreateAPIToken(userID, description string) (token *users.APIToken, err error) {
	t.timeRequest("CreateAPIToken", func() error {
		token, err = t.d.CreateAPIToken(userID, description)
		return err
	})
	return
}

func (t timed) DeleteAPIToken(userID, token string) error {
	return t.timeRequest("DeleteAPIToken", func() error {
		return t.d.DeleteAPIToken(userID, token)
	})
}

func (t timed) InviteUser(email, orgExternalID string) (u *users.User, created bool, err error) {
	t.timeRequest("InviteUser", func() error {
		u, created, err = t.d.InviteUser(email, orgExternalID)
		return err
	})
	return
}

func (t timed) RemoveUserFromOrganization(orgExternalID, email string) error {
	return t.timeRequest("RemoveUserFromOrganization", func() error {
		return t.d.RemoveUserFromOrganization(orgExternalID, email)
	})
}

func (t timed) ListUsers() (us []*users.User, err error) {
	t.timeRequest("ListUsers", func() error {
		us, err = t.d.ListUsers()
		return err
	})
	return
}

func (t timed) ListOrganizations() (os []*users.Organization, err error) {
	t.timeRequest("ListOrganizations", func() error {
		os, err = t.d.ListOrganizations()
		return err
	})
	return
}

func (t timed) ListOrganizationUsers(orgExternalID string) (us []*users.User, err error) {
	t.timeRequest("ListOrganizationUsers", func() error {
		us, err = t.d.ListOrganizationUsers(orgExternalID)
		return err
	})
	return
}

func (t timed) ListOrganizationsForUserIDs(userIDs ...string) (os []*users.Organization, err error) {
	t.timeRequest("ListOrganizationsForUserIDs", func() error {
		os, err = t.d.ListOrganizationsForUserIDs(userIDs...)
		return err
	})
	return
}

func (t timed) ListLoginsForUserIDs(userIDs ...string) (ls []*login.Login, err error) {
	t.timeRequest("ListLoginsForUserIDs", func() error {
		ls, err = t.d.ListLoginsForUserIDs(userIDs...)
		return err
	})
	return
}

func (t timed) ListAPITokensForUserIDs(userIDs ...string) (ts []*users.APIToken, err error) {
	t.timeRequest("ListAPITokensForUserIDs", func() error {
		ts, err = t.d.ListAPITokensForUserIDs(userIDs...)
		return err
	})
	return
}

func (t timed) SetUserAdmin(id string, value bool) error {
	return t.timeRequest("SetUserAdmin", func() error {
		return t.d.SetUserAdmin(id, value)
	})
}

func (t timed) SetUserToken(id, token string) error {
	return t.timeRequest("SetUserToken", func() error {
		return t.d.SetUserToken(id, token)
	})
}

func (t timed) SetUserFirstLoginAt(id string) error {
	return t.timeRequest("SetUserFirstLoginAt", func() error {
		return t.d.SetUserFirstLoginAt(id)
	})
}

func (t timed) GenerateOrganizationExternalID() (s string, err error) {
	t.timeRequest("GenerateOrganizationExternalID", func() error {
		s, err = t.d.GenerateOrganizationExternalID()
		return err
	})
	return
}

func (t timed) CreateOrganization(ownerID, externalID, name string) (o *users.Organization, err error) {
	t.timeRequest("CreateOrganization", func() error {
		o, err = t.d.CreateOrganization(ownerID, externalID, name)
		return err
	})
	return
}

func (t timed) FindOrganizationByProbeToken(probeToken string) (o *users.Organization, err error) {
	t.timeRequest("FindOrganizationByProbeToken", func() error {
		o, err = t.d.FindOrganizationByProbeToken(probeToken)
		return err
	})
	return
}

func (t timed) RenameOrganization(externalID, name string) error {
	return t.timeRequest("RenameOrganization", func() error {
		return t.d.RenameOrganization(externalID, name)
	})
}

func (t timed) OrganizationExists(externalID string) (b bool, err error) {
	t.timeRequest("OrganizationExists", func() error {
		b, err = t.d.OrganizationExists(externalID)
		return err
	})
	return
}

func (t timed) GetOrganizationName(externalID string) (name string, err error) {
	t.timeRequest("GetOrganizationName", func() error {
		name, err = t.d.GetOrganizationName(externalID)
		return err
	})
	return
}

func (t timed) DeleteOrganization(externalID string) error {
	return t.timeRequest("DeleteOrganization", func() error {
		return t.d.DeleteOrganization(externalID)
	})
}

func (t timed) AddFeatureFlag(externalID string, featureFlag string) error {
	return t.timeRequest("AddFeatureFlag", func() error {
		return t.d.AddFeatureFlag(externalID, featureFlag)
	})
}

func (t timed) Close() error {
	return t.timeRequest("Close", func() error {
		return t.d.Close()
	})
}
