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
		err = users.ErrInvalidAuthenticationData
	}
	if err != nil {
		return nil, err
	}
	for _, org := range organizations {
		if strings.ToLower(org.ExternalID) == strings.ToLower(req.OrgExternalID) {
			if org.DenyUIFeatures && req.AuthorizeForUIFeatures {
				return nil, users.ErrOrgUIFeaturesDisabled
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

// LookupAdmin authenticates a cookie for admin access.
func (a *usersServer) LookupAdmin(ctx context.Context, req *users.LookupAdminRequest) (*users.LookupAdminResponse, error) {
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

// LookupUsingToken authenticates a token for access to an org.
func (a *usersServer) LookupUsingToken(ctx context.Context, req *users.LookupUsingTokenRequest) (*users.LookupUsingTokenResponse, error) {
	o, err := a.db.FindOrganizationByProbeToken(ctx, req.Token)
	if err == users.ErrNotFound {
		err = users.ErrInvalidAuthenticationData
	}
	if err != nil {
		return nil, err
	}
	if o.DenyTokenAuth {
		return nil, users.ErrOrgTokenAuthDisabled
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
	return &users.LookupUserResponse{
		UserID: session.UserID,
	}, nil
}

func (a *usersServer) GetOrganizations(ctx context.Context, req *users.GetOrganizationsRequest) (*users.GetOrganizationsResponse, error) {
	organizations, err := a.db.ListOrganizations(ctx)
	if err != nil {
		return nil, err
	}

	result := &users.GetOrganizationsResponse{}
	for _, org := range organizations {
		result.Organizations = append(result.Organizations, users.Organization{
			ID:             org.ID,
			ExternalID:     org.ExternalID,
			Name:           org.Name,
			ProbeToken:     org.ProbeToken,
			CreatedAt:      org.CreatedAt,
			FeatureFlags:   org.FeatureFlags,
			DenyUIFeatures: org.DenyUIFeatures,
			DenyTokenAuth:  org.DenyTokenAuth,
		})
	}
	return result, nil
}

func (a *usersServer) GetOrganization(ctx context.Context, req *users.GetOrganizationRequest) (*users.GetOrganizationResponse, error) {
	organization, err := a.db.FindOrganizationByID(ctx, req.ExternalID)
	if err != nil {
		return nil, err
	}

	return &users.GetOrganizationResponse{
		Organization: users.Organization{
			ID:             organization.ID,
			ExternalID:     organization.ExternalID,
			Name:           organization.Name,
			ProbeToken:     organization.ProbeToken,
			CreatedAt:      organization.CreatedAt,
			FeatureFlags:   organization.FeatureFlags,
			DenyUIFeatures: organization.DenyUIFeatures,
			DenyTokenAuth:  organization.DenyTokenAuth,
		},
	}, nil
}

func (a *usersServer) SetOrganizationFlag(ctx context.Context, req *users.SetOrganizationFlagRequest) (*users.SetOrganizationFlagResponse, error) {
	var err error
	switch req.Flag {
	case "DenyUIFeatures":
		err = a.db.SetOrganizationDenyUIFeatures(ctx, req.ExternalID, req.Value)
	case "DenyTokenAuth":
		err = a.db.SetOrganizationDenyTokenAuth(ctx, req.ExternalID, req.Value)
	default:
		err = fmt.Errorf("Invalid flag: %v", req.Flag)
	}
	if err != nil {
		return nil, err
	}
	return &users.SetOrganizationFlagResponse{}, nil
}

func (a *usersServer) GetUser(ctx context.Context, req *users.GetUserRequest) (*users.GetUserResponse, error) {
	user, err := a.db.FindUserByID(ctx, req.UserID)
	if err != nil {
		return nil, err
	}

	return &users.GetUserResponse{
		User: users.User{
			ID:    user.ID,
			Email: user.Email,
		},
	}, nil
}
