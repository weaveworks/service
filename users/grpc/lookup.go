package grpc

import (
	"fmt"
	"strings"
	"time"

	"golang.org/x/net/context"

	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/db"
	"github.com/weaveworks/service/users/db/filter"
	"github.com/weaveworks/service/users/sessions"
)

const (
	billingFlag = "billing"
)

// usersServer implements users.UsersServer
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

func authorizeAction(action users.AuthorizedAction, org *users.Organization) error {
	// TODO: Rename DenyUIFeatures & DenyTokenAuth https://github.com/weaveworks/service/issues/1256
	switch action {
	case users.INSTANCE_DATA_ACCESS:
		if org.DenyUIFeatures {
			return users.ErrInstanceDataAccessDenied
		}
	case users.INSTANCE_DATA_UPLOAD:
		if org.DenyTokenAuth {
			return users.ErrInstanceDataUploadDenied
		}
	}
	// TODO: Future - consider switching to default-deny
	return nil
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
	err = authorizeAction(req.AuthorizeFor, o)
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
	return &users.LookupUserResponse{
		UserID: session.UserID,
	}, nil
}

func (a *usersServer) GetOrganizations(ctx context.Context, req *users.GetOrganizationsRequest) (*users.GetOrganizationsResponse, error) {
	organizations, err := a.db.ListOrganizations(ctx, filter.Organization{Search: req.Query, Page: req.PageNumber})
	if err != nil {
		return nil, err
	}

	result := &users.GetOrganizationsResponse{}
	for _, org := range organizations {
		result.Organizations = append(result.Organizations, *org)
	}
	return result, nil
}

func (a *usersServer) GetBillableOrganizations(ctx context.Context, req *users.GetBillableOrganizationsRequest) (*users.GetBillableOrganizationsResponse, error) {
	// While billing is in development, only pick orgs with ff `billing`
	organizations, err := a.db.ListOrganizations(ctx, filter.Organization{FeatureFlags: []string{billingFlag}})
	if err != nil {
		return nil, err
	}

	result := &users.GetBillableOrganizationsResponse{}
	for _, org := range organizations {
		// TODO: Move this filtering into the database layer.
		if org.InTrialPeriod(req.Now) {
			// Still in trial period, so not billable.
			continue
		}
		result.Organizations = append(result.Organizations, *org)
	}
	return result, nil
}

func (a *usersServer) GetTrialOrganizations(ctx context.Context, req *users.GetTrialOrganizationsRequest) (*users.GetTrialOrganizationsResponse, error) {
	// While billing is in development, only pick orgs with ff `billing`
	organizations, err := a.db.ListOrganizations(ctx, filter.Organization{FeatureFlags: []string{billingFlag}})
	if err != nil {
		return nil, err
	}

	result := &users.GetTrialOrganizationsResponse{}
	for _, org := range organizations {
		// TODO: Move this filtering into the database layer.
		if !org.InTrialPeriod(req.Now) {
			continue
		}
		result.Organizations = append(result.Organizations, *org)
	}
	return result, nil
}

func (a *usersServer) GetDelinquentOrganizations(ctx context.Context, req *users.GetDelinquentOrganizationsRequest) (*users.GetDelinquentOrganizationsResponse, error) {
	// While billing is in development, only pick orgs with ff `billing`
	organizations, err := a.db.ListOrganizations(ctx, filter.Organization{FeatureFlags: []string{billingFlag}})
	if err != nil {
		return nil, err
	}

	result := &users.GetDelinquentOrganizationsResponse{}
	for _, org := range organizations {
		// TODO: Move this filtering into the database layer.
		if org.InTrialPeriod(req.Now) {
			continue
		}
		// Not Zuora account means the organization hasn't supplied means for payment
		if org.ZuoraAccountNumber != "" {
			continue
		}
		result.Organizations = append(result.Organizations, *org)
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
			ID:                    organization.ID,
			ExternalID:            organization.ExternalID,
			Name:                  organization.Name,
			ProbeToken:            organization.ProbeToken,
			CreatedAt:             organization.CreatedAt,
			FeatureFlags:          organization.FeatureFlags,
			DenyUIFeatures:        organization.DenyUIFeatures,
			DenyTokenAuth:         organization.DenyTokenAuth,
			FirstSeenConnectedAt:  organization.FirstSeenConnectedAt,
			Platform:              organization.Platform,
			Environment:           organization.Environment,
			ZuoraAccountNumber:    organization.ZuoraAccountNumber,
			ZuoraAccountCreatedAt: organization.ZuoraAccountCreatedAt,
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

func (a *usersServer) SetOrganizationZuoraAccount(ctx context.Context, req *users.SetOrganizationZuoraAccountRequest) (*users.SetOrganizationZuoraAccountResponse, error) {
	var createdAt time.Time
	if req.CreatedAt == nil {
		createdAt = time.Now()
	} else {
		createdAt = *req.CreatedAt
	}
	err := a.db.SetOrganizationZuoraAccount(ctx, req.ExternalID, req.Number, &createdAt)
	if err != nil {
		return nil, err
	}
	return &users.SetOrganizationZuoraAccountResponse{}, nil
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
