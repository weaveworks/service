package grpc

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/weaveworks/service/common/featureflag"
	"github.com/weaveworks/service/common/orgs"

	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/db"
	"github.com/weaveworks/service/users/db/filter"
	"github.com/weaveworks/service/users/emailer"
	"github.com/weaveworks/service/users/sessions"
	"golang.org/x/net/context"
)

// AuthUserForOrg authenticates a cookie for access to an org by external ID.
func (a *usersServer) AuthUserForOrg(ctx context.Context, req *users.LookupOrgRequest) (*users.LookupOrgResponse, error) {
	session, err := a.sessions.Decode(req.Cookie)
	if err != nil {
		return nil, err
	}

	organizations, err := a.db.ListOrganizationsForUserIDs(ctx, session.UserID)
	if err == users.ErrNotFound {
		err = users.ErrInvalidAuthenticationData
	}
	if err != nil {
		return nil, err
	}
	for _, org := range organizations {
		if strings.ToLower(org.ExternalID) == strings.ToLower(req.OrgExternalID) {
			err := authorizeAction(req.AuthorizeFor, org)
			if err != nil {
				return nil, err
			}

			return &users.LookupOrgResponse{
				OrganizationID: org.ID,
				UserID:         session.UserID,
				FeatureFlags:   org.FeatureFlags,
			}, nil
		}
	}
	return nil, users.ErrInvalidAuthenticationData
}

// AuthUserForAdmin authenticates a cookie for admin access.
func (a *usersServer) AuthUserForAdmin(ctx context.Context, req *users.LookupAdminRequest) (*users.LookupAdminResponse, error) {
	session, err := a.sessions.Decode(req.Cookie)
	if err != nil {
		return nil, err
	}
	u, err := a.db.FindUserByID(ctx, session.UserID)
	if err == users.ErrNotFound {
		err = users.ErrInvalidAuthenticationData
	}
	if err != nil {
		return nil, err
	}
	if !u.Admin {
		return nil, users.ErrInvalidAuthenticationData
	}
	return &users.LookupAdminResponse{
		AdminID: u.ID,
	}, nil
}

// AuthUserForOrg authenticates a token for access to an org.
func (a *usersServer) AuthUserForOrg(ctx context.Context, req *users.LookupUsingTokenRequest) (*users.LookupUsingTokenResponse, error) {
	o, err := a.db.FindOrganizationByProbeToken(ctx, req.Token)
	if err == users.ErrNotFound {
		err = users.ErrInvalidAuthenticationData
	}
	if err != nil {
		return nil, err
	}
	err = authorizeAction(req.AuthorizeFor, o)
	if err != nil {
		return nil, err
	}
	return &users.LookupUsingTokenResponse{
		OrganizationID: o.ID,
		FeatureFlags:   o.FeatureFlags,
	}, nil
}

// AuthUser authenticates a cookie.
func (a *usersServer) AuthUser(ctx context.Context, req *users.LookupUserRequest) (*users.LookupUserResponse, error) {
	session, err := a.sessions.Decode(req.Cookie)
	if err != nil {
		return nil, err
	}
	return &users.LookupUserResponse{
		UserID: session.UserID,
	}, nil
}

// AuthWebhookSecretForOrg gets the webhook given the external org ID and the secret ID of the webhook.
func (a *usersServer) AuthWebhookSecretForOrg(ctx context.Context, req *users.LookupOrganizationWebhookUsingSecretIDRequest) (*users.LookupOrganizationWebhookUsingSecretIDResponse, error) {
	webhook, err := a.db.FindOrganizationWebhookBySecretID(ctx, req.SecretID)
	if err == users.ErrNotFound {
		err = httpgrpc.Errorf(http.StatusNotFound, "Webhook does not exist.")
	}
	if err != nil {
		return nil, err
	}
	return &users.LookupOrganizationWebhookUsingSecretIDResponse{
		Webhook: webhook,
	}, nil
}
