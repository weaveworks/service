package grpc

import (
	"fmt"
	"strings"

	"golang.org/x/net/context"

	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/db"
	"github.com/weaveworks/service/users/sessions"
)

type usersServer struct {
	sessions sessions.Store
	db       db.DB
}

// New makes a new users.UsersServer
func New(sessions sessions.Store, db db.DB) users.UsersServer {
	return &usersServer{
		sessions: sessions,
		db:       db,
	}
}

// LookupOrg authenticates a cookie for access to an org by extenal ID.
func (a *usersServer) LookupOrg(ctx context.Context, req *users.LookupOrgRequest) (*users.LookupOrgResponse, error) {
	session, err := a.sessions.Decode(req.Cookie)
	if err != nil {
		return nil, err
	}

	organizations, err := a.db.ListOrganizationsForUserIDs(ctx, session.UserID)
	if err == users.ErrNotFound {
		err = users.NewInvalidAuthenticationDataError(fmt.Errorf("userID %v not found", session.UserID))
	}
	if err != nil {
		return nil, err
	}
	for _, org := range organizations {
		if strings.ToLower(org.ExternalID) == strings.ToLower(req.OrgExternalID) {
			return &users.LookupOrgResponse{
				OrganizationID: org.ID,
				UserID:         session.UserID,
				FeatureFlags:   org.FeatureFlags,
			}, nil
		}
	}
	return nil, users.NewInvalidAuthenticationDataError(
		fmt.Errorf("userID %v not in organization %v", session.UserID, req.OrgExternalID))
}

// LookupAdmin authenticates a cookie for admin access.
func (a *usersServer) LookupAdmin(ctx context.Context, req *users.LookupAdminRequest) (*users.LookupAdminResponse, error) {
	session, err := a.sessions.Decode(req.Cookie)
	if err != nil {
		return nil, err
	}
	u, err := a.db.FindUserByID(ctx, session.UserID)
	if err == users.ErrNotFound {
		err = users.NewInvalidAuthenticationDataError(fmt.Errorf("userID %v not found", session.UserID))
	}
	if err != nil {
		return nil, err
	}
	if !u.Admin {
		return nil, users.NewInvalidAuthenticationDataError(fmt.Errorf("userID %v not an admin", session.UserID))
	}
	return &users.LookupAdminResponse{
		AdminID: u.ID,
	}, nil
}

// LookupUsingToken authenticates a token for access to an org.
func (a *usersServer) LookupUsingToken(ctx context.Context, req *users.LookupUsingTokenRequest) (*users.LookupUsingTokenResponse, error) {
	o, err := a.db.FindOrganizationByProbeToken(ctx, req.Token)
	if err == users.ErrNotFound {
		err = users.NewInvalidAuthenticationDataError(fmt.Errorf("no organization for probe token %v", req.Token))
	}
	if err != nil {
		return nil, err
	}
	return &users.LookupUsingTokenResponse{
		OrganizationID: o.ID,
		FeatureFlags:   o.FeatureFlags,
	}, nil
}

// LookupUser authenticates a cookie.
func (a *usersServer) LookupUser(ctx context.Context, req *users.LookupUserRequest) (*users.LookupUserResponse, error) {
	session, err := a.sessions.Decode(req.Cookie)
	if err != nil {
		return nil, err
	}
	return &users.LookupUserResponse{session.UserID}, nil
}
