package db

import (
	"context"
	"encoding/json"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/weaveworks/common/instrument"

	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/db/filter"
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

func (t timed) timeRequest(ctx context.Context, method string, f func(context.Context) error) error {
	return instrument.TimeRequestHistogramStatus(ctx, method, t.Duration, t.errorCode, f)
}

func (t timed) CreateUser(ctx context.Context, email string) (u *users.User, err error) {
	t.timeRequest(ctx, "CreateUser", func(ctx context.Context) error {
		u, err = t.d.CreateUser(ctx, email)
		return err
	})
	return
}

func (t timed) FindUserByID(ctx context.Context, id string) (u *users.User, err error) {
	t.timeRequest(ctx, "FindUserByID", func(ctx context.Context) error {
		u, err = t.d.FindUserByID(ctx, id)
		return err
	})
	return
}

func (t timed) FindUserByEmail(ctx context.Context, email string) (u *users.User, err error) {
	t.timeRequest(ctx, "FindUserByEmail", func(ctx context.Context) error {
		u, err = t.d.FindUserByEmail(ctx, email)
		return err
	})
	return
}

func (t timed) FindUserByLogin(ctx context.Context, provider, id string) (u *users.User, err error) {
	t.timeRequest(ctx, "FindUserByLogin", func(ctx context.Context) error {
		u, err = t.d.FindUserByLogin(ctx, provider, id)
		return err
	})
	return
}

func (t timed) UserIsMemberOf(ctx context.Context, userID, orgExternalID string) (b bool, err error) {
	t.timeRequest(ctx, "UserIsMemberOf", func(ctx context.Context) error {
		b, err = t.d.UserIsMemberOf(ctx, userID, orgExternalID)
		return err
	})
	return
}

func (t timed) AddLoginToUser(ctx context.Context, userID, provider, id string, session json.RawMessage) error {
	return t.timeRequest(ctx, "AddLoginToUser", func(ctx context.Context) error {
		return t.d.AddLoginToUser(ctx, userID, provider, id, session)
	})
}

func (t timed) DetachLoginFromUser(ctx context.Context, userID, provider string) error {
	return t.timeRequest(ctx, "DetachLoginFromUser", func(ctx context.Context) error {
		return t.d.DetachLoginFromUser(ctx, userID, provider)
	})
}

func (t timed) InviteUser(ctx context.Context, email, orgExternalID string) (u *users.User, created bool, err error) {
	t.timeRequest(ctx, "InviteUser", func(ctx context.Context) error {
		u, created, err = t.d.InviteUser(ctx, email, orgExternalID)
		return err
	})
	return
}

func (t timed) RemoveUserFromOrganization(ctx context.Context, orgExternalID, email string) error {
	return t.timeRequest(ctx, "RemoveUserFromOrganization", func(ctx context.Context) error {
		return t.d.RemoveUserFromOrganization(ctx, orgExternalID, email)
	})
}

func (t timed) ListUsers(ctx context.Context, f filter.User) (us []*users.User, err error) {
	t.timeRequest(ctx, "ListUsers", func(ctx context.Context) error {
		us, err = t.d.ListUsers(ctx, f)
		return err
	})
	return
}

func (t timed) ListOrganizations(ctx context.Context, f filter.Organization) (os []*users.Organization, err error) {
	t.timeRequest(ctx, "ListOrganizations", func(ctx context.Context) error {
		os, err = t.d.ListOrganizations(ctx, f)
		return err
	})
	return
}

func (t timed) ListOrganizationUsers(ctx context.Context, orgExternalID string) (us []*users.User, err error) {
	t.timeRequest(ctx, "ListOrganizationUsers", func(ctx context.Context) error {
		us, err = t.d.ListOrganizationUsers(ctx, orgExternalID)
		return err
	})
	return
}

func (t timed) ListOrganizationsForUserIDs(ctx context.Context, userIDs ...string) (os []*users.Organization, err error) {
	t.timeRequest(ctx, "ListOrganizationsForUserIDs", func(ctx context.Context) error {
		os, err = t.d.ListOrganizationsForUserIDs(ctx, userIDs...)
		return err
	})
	return
}

func (t timed) ListLoginsForUserIDs(ctx context.Context, userIDs ...string) (ls []*login.Login, err error) {
	t.timeRequest(ctx, "ListLoginsForUserIDs", func(ctx context.Context) error {
		ls, err = t.d.ListLoginsForUserIDs(ctx, userIDs...)
		return err
	})
	return
}

func (t timed) SetUserAdmin(ctx context.Context, id string, value bool) error {
	return t.timeRequest(ctx, "SetUserAdmin", func(ctx context.Context) error {
		return t.d.SetUserAdmin(ctx, id, value)
	})
}

func (t timed) SetUserToken(ctx context.Context, id, token string) error {
	return t.timeRequest(ctx, "SetUserToken", func(ctx context.Context) error {
		return t.d.SetUserToken(ctx, id, token)
	})
}

func (t timed) SetUserFirstLoginAt(ctx context.Context, id string) error {
	return t.timeRequest(ctx, "SetUserFirstLoginAt", func(ctx context.Context) error {
		return t.d.SetUserFirstLoginAt(ctx, id)
	})
}

func (t timed) GenerateOrganizationExternalID(ctx context.Context) (s string, err error) {
	t.timeRequest(ctx, "GenerateOrganizationExternalID", func(ctx context.Context) error {
		s, err = t.d.GenerateOrganizationExternalID(ctx)
		return err
	})
	return
}

func (t timed) CreateOrganization(ctx context.Context, ownerID, externalID, name, token string) (o *users.Organization, err error) {
	t.timeRequest(ctx, "CreateOrganization", func(ctx context.Context) error {
		o, err = t.d.CreateOrganization(ctx, ownerID, externalID, name, token)
		return err
	})
	return
}

func (t timed) FindOrganizationByProbeToken(ctx context.Context, probeToken string) (o *users.Organization, err error) {
	t.timeRequest(ctx, "FindOrganizationByProbeToken", func(ctx context.Context) error {
		o, err = t.d.FindOrganizationByProbeToken(ctx, probeToken)
		return err
	})
	return
}

func (t timed) FindOrganizationByID(ctx context.Context, externalID string) (o *users.Organization, err error) {
	t.timeRequest(ctx, "FindOrganizationByID", func(ctx context.Context) error {
		o, err = t.d.FindOrganizationByID(ctx, externalID)
		return err
	})
	return
}

func (t timed) FindOrganizationByGCPAccountID(ctx context.Context, accountID string) (o *users.Organization, err error) {
	t.timeRequest(ctx, "FindOrganizationByGCPAccountID", func(ctx context.Context) error {
		o, err = t.d.FindOrganizationByGCPAccountID(ctx, accountID)
		return err
	})
	return
}

func (t timed) UpdateOrganization(ctx context.Context, externalID string, update users.OrgWriteView) error {
	return t.timeRequest(ctx, "UpdateOrganization", func(ctx context.Context) error {
		return t.d.UpdateOrganization(ctx, externalID, update)
	})
}

func (t timed) OrganizationExists(ctx context.Context, externalID string) (b bool, err error) {
	t.timeRequest(ctx, "OrganizationExists", func(ctx context.Context) error {
		b, err = t.d.OrganizationExists(ctx, externalID)
		return err
	})
	return
}

func (t timed) ExternalIDUsed(ctx context.Context, externalID string) (b bool, err error) {
	t.timeRequest(ctx, "ExternalIDUsed", func(ctx context.Context) error {
		b, err = t.d.ExternalIDUsed(ctx, externalID)
		return err
	})
	return
}

func (t timed) GetOrganizationName(ctx context.Context, externalID string) (name string, err error) {
	t.timeRequest(ctx, "GetOrganizationName", func(ctx context.Context) error {
		name, err = t.d.GetOrganizationName(ctx, externalID)
		return err
	})
	return
}

func (t timed) DeleteOrganization(ctx context.Context, externalID string) error {
	return t.timeRequest(ctx, "DeleteOrganization", func(ctx context.Context) error {
		return t.d.DeleteOrganization(ctx, externalID)
	})
}

func (t timed) AddFeatureFlag(ctx context.Context, externalID string, featureFlag string) error {
	return t.timeRequest(ctx, "AddFeatureFlag", func(ctx context.Context) error {
		return t.d.AddFeatureFlag(ctx, externalID, featureFlag)
	})
}

func (t timed) SetFeatureFlags(ctx context.Context, externalID string, featureFlags []string) error {
	return t.timeRequest(ctx, "SetFeatureFlags", func(ctx context.Context) error {
		return t.d.SetFeatureFlags(ctx, externalID, featureFlags)
	})
}

func (t timed) SetOrganizationRefuseDataAccess(ctx context.Context, externalID string, value bool) error {
	return t.timeRequest(ctx, "SetOrganizationRefuseDataAccess", func(ctx context.Context) error {
		return t.d.SetOrganizationRefuseDataAccess(ctx, externalID, value)
	})
}

func (t timed) SetOrganizationRefuseDataUpload(ctx context.Context, externalID string, value bool) error {
	return t.timeRequest(ctx, "SetOrganizationRefuseDataUpload", func(ctx context.Context) error {
		return t.d.SetOrganizationRefuseDataUpload(ctx, externalID, value)
	})
}

func (t timed) SetOrganizationFirstSeenConnectedAt(ctx context.Context, externalID string, value *time.Time) error {
	return t.timeRequest(ctx, "SetOrganizationFirstSeenConnectedAt", func(ctx context.Context) error {
		return t.d.SetOrganizationFirstSeenConnectedAt(ctx, externalID, value)
	})
}

func (t timed) SetOrganizationZuoraAccount(ctx context.Context, externalID, number string, createdAt *time.Time) error {
	return t.timeRequest(ctx, "SetOrganizationZuoraAccount", func(ctx context.Context) error {
		return t.d.SetOrganizationZuoraAccount(ctx, externalID, number, createdAt)
	})
}

func (t timed) ListMemberships(ctx context.Context) (memberships []users.Membership, err error) {
	t.timeRequest(ctx, "ListMemberships", func(ctx context.Context) error {
		memberships, err = t.d.ListMemberships(ctx)
		return err
	})
	return
}

func (t timed) Close(ctx context.Context) error {
	return t.timeRequest(ctx, "Close", func(ctx context.Context) error {
		return t.d.Close(ctx)
	})
}
