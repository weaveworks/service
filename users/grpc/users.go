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

// usersServer implements users.UsersServer
type usersServer struct {
	sessions sessions.Store
	db       db.DB
	emailer  emailer.Emailer
}

// New makes a new users.UsersServer
func New(sessions sessions.Store, db db.DB, emailer emailer.Emailer) users.UsersServer {
	return &usersServer{
		sessions: sessions,
		db:       db,
		emailer:  emailer,
	}
}

func authorizeAction(action users.AuthorizedAction, org *users.Organization) error {
	switch action {
	case users.INSTANCE_DATA_ACCESS:
		if org.RefuseDataAccess {
			return users.ErrInstanceDataAccessDenied(org.ExternalID, org.RefuseDataReason)
		}
	case users.INSTANCE_DATA_UPLOAD:
		if org.RefuseDataUpload {
			return users.ErrInstanceDataUploadDenied(org.ExternalID, org.RefuseDataReason)
		}
	}
	// TODO: Future - consider switching to default-deny
	return nil
}

/** LEGACY remove these after all clients in production are upgraded **/

// LookupOrg authenticates a cookie for access to an org by external ID.
func (a *usersServer) LookupOrg(ctx context.Context, req *users.LookupOrgRequest) (*users.LookupOrgResponse, error) {
	return a.AuthUserForOrg(ctx, req)
}

// LookupAdmin authenticates a cookie for admin access.
func (a *usersServer) LookupAdmin(ctx context.Context, req *users.LookupAdminRequest) (*users.LookupAdminResponse, error) {
	return a.AuthUserForAdmin(ctx, req)
}

// LookupUsingToken authenticates a token for access to an org.
func (a *usersServer) LookupUsingToken(ctx context.Context, req *users.LookupUsingTokenRequest) (*users.LookupUsingTokenResponse, error) {
	return a.AuthTokenForOrg(ctx, req)
}

// LookupUser authenticates a cookie.
func (a *usersServer) LookupUser(ctx context.Context, req *users.LookupUserRequest) (*users.LookupUserResponse, error) {
	return a.AuthUser(ctx, req)
}

/** END LEGACY **/

func (a *usersServer) GetOrganizations(ctx context.Context, req *users.GetOrganizationsRequest) (*users.GetOrganizationsResponse, error) {
	fs := []filter.Filter{}
	if req.Query != "" {
		fs = append(fs, filter.ExternalID(req.Query))
	}
	organizations, err := a.db.ListOrganizations(ctx, filter.And(fs...), uint64(req.PageNumber))
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
	organizations, err := a.db.ListOrganizations(
		ctx,
		filter.And(
			filter.Or(filter.ZuoraAccount(true), filter.GCPSubscription(true)),
			filter.TrialExpiredBy(req.Now),
			// While billing is in development, only pick orgs with ff `billing`
			filter.HasFeatureFlag(featureflag.Billing),
		),
		0,
	)
	if err != nil {
		return nil, err
	}

	result := &users.GetBillableOrganizationsResponse{}
	for _, org := range organizations {
		result.Organizations = append(result.Organizations, *org)
	}
	return result, nil
}

func (a *usersServer) GetTrialOrganizations(ctx context.Context, req *users.GetTrialOrganizationsRequest) (*users.GetTrialOrganizationsResponse, error) {
	organizations, err := a.db.ListOrganizations(
		ctx,
		filter.And(
			filter.GCP(false), // Trial is never active for GCP instances but we still make sure here.
			filter.TrialActiveAt(req.Now),
			filter.HasFeatureFlag(featureflag.Billing),
		),
		0,
	)
	if err != nil {
		return nil, err
	}

	result := &users.GetTrialOrganizationsResponse{}
	for _, org := range organizations {
		result.Organizations = append(result.Organizations, *org)
	}
	return result, nil
}

func (a *usersServer) GetDelinquentOrganizations(ctx context.Context, req *users.GetDelinquentOrganizationsRequest) (*users.GetDelinquentOrganizationsResponse, error) {
	// While billing is in development, only pick orgs with ff `billing`
	organizations, err := a.db.ListOrganizations(
		ctx,
		orgs.DelinquentFilter(req.Now),
		0,
	)
	if err != nil {
		return nil, err
	}

	result := &users.GetDelinquentOrganizationsResponse{}
	for _, org := range organizations {
		result.Organizations = append(result.Organizations, *org)
	}
	return result, nil
}

func (a *usersServer) GetOrganization(ctx context.Context, req *users.GetOrganizationRequest) (*users.GetOrganizationResponse, error) {
	var organization *users.Organization
	var err error

	if req.GetExternalID() != "" {
		organization, err = a.db.FindOrganizationByID(ctx, req.GetExternalID())
	} else if req.GetGCPExternalAccountID() != "" {
		organization, err = a.db.FindOrganizationByGCPExternalAccountID(ctx, req.GetGCPExternalAccountID())
	} else if req.GetInternalID() != "" {
		organization, err = a.db.FindOrganizationByInternalID(ctx, req.GetInternalID())
	} else {
		err = errors.New("ID not set")
	}

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
			RefuseDataAccess:      organization.RefuseDataAccess,
			RefuseDataUpload:      organization.RefuseDataUpload,
			FirstSeenConnectedAt:  organization.FirstSeenConnectedAt,
			Platform:              organization.Platform,
			Environment:           organization.Environment,
			TrialExpiresAt:        organization.TrialExpiresAt,
			ZuoraAccountNumber:    organization.ZuoraAccountNumber,
			ZuoraAccountCreatedAt: organization.ZuoraAccountCreatedAt,
		},
	}, nil
}

func (a *usersServer) SetOrganizationFlag(ctx context.Context, req *users.SetOrganizationFlagRequest) (*users.SetOrganizationFlagResponse, error) {
	var err error
	switch req.Flag {
	case orgs.RefuseDataAccess:
		err = a.db.SetOrganizationRefuseDataAccess(ctx, req.ExternalID, req.Value)
	case orgs.RefuseDataUpload:
		err = a.db.SetOrganizationRefuseDataUpload(ctx, req.ExternalID, req.Value)
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

func (a *usersServer) NotifyTrialPendingExpiry(ctx context.Context, req *users.NotifyTrialPendingExpiryRequest) (*users.NotifyTrialPendingExpiryResponse, error) {
	// Make sure the organization exists
	org, err := a.db.FindOrganizationByID(ctx, req.ExternalID)
	if err != nil {
		return nil, err
	}

	// Notify all users
	members, err := a.db.ListOrganizationUsers(ctx, req.ExternalID)
	if err != nil {
		return nil, err
	}
	err = a.emailer.TrialPendingExpiryEmail(members, req.ExternalID, org.Name, org.TrialExpiresAt)
	if err != nil {
		return nil, err
	}

	// Persist sent date in db
	now := time.Now()
	_, err = a.db.UpdateOrganization(ctx, req.ExternalID, users.OrgWriteView{TrialPendingExpiryNotifiedAt: &now})

	return &users.NotifyTrialPendingExpiryResponse{}, err
}

func (a *usersServer) NotifyTrialExpired(ctx context.Context, req *users.NotifyTrialExpiredRequest) (*users.NotifyTrialExpiredResponse, error) {
	// Make sure the organization exists
	org, err := a.db.FindOrganizationByID(ctx, req.ExternalID)
	if err != nil {
		return nil, err
	}

	// Notify all users
	members, err := a.db.ListOrganizationUsers(ctx, req.ExternalID)
	if err != nil {
		return nil, err
	}
	err = a.emailer.TrialExpiredEmail(members, req.ExternalID, org.Name)
	if err != nil {
		return nil, err
	}

	// Persist sent date in db
	now := time.Now()
	_, err = a.db.UpdateOrganization(ctx, req.ExternalID, users.OrgWriteView{TrialExpiredNotifiedAt: &now})

	return &users.NotifyTrialExpiredResponse{}, err
}

func (a *usersServer) NotifyRefuseDataUpload(ctx context.Context, req *users.NotifyRefuseDataUploadRequest) (*users.NotifyRefuseDataUploadResponse, error) {
	// Make sure the organization exists
	org, err := a.db.FindOrganizationByID(ctx, req.ExternalID)
	if err != nil {
		return nil, err
	}

	// Notify all users
	members, err := a.db.ListOrganizationUsers(ctx, req.ExternalID)
	if err != nil {
		return nil, err
	}
	err = a.emailer.RefuseDataUploadEmail(members, req.ExternalID, org.Name)
	if err != nil {
		return nil, err
	}

	return &users.NotifyRefuseDataUploadResponse{}, err
}

func (a *usersServer) GetGCP(ctx context.Context, req *users.GetGCPRequest) (*users.GetGCPResponse, error) {
	gcp, err := a.db.FindGCP(ctx, req.ExternalAccountID)
	if err != nil {
		return nil, err
	}
	return &users.GetGCPResponse{GCP: *gcp}, nil
}

func (a *usersServer) UpdateGCP(ctx context.Context, req *users.UpdateGCPRequest) (*users.UpdateGCPResponse, error) {
	err := a.db.UpdateGCP(ctx, req.GCP.ExternalAccountID, req.GCP.ConsumerID, req.GCP.SubscriptionName, req.GCP.SubscriptionLevel, req.GCP.SubscriptionStatus)
	if err != nil {
		return nil, err

	}
	return &users.UpdateGCPResponse{}, nil
}

func (a *usersServer) GetSummary(ctx context.Context, _ *users.Empty) (*users.Summary, error) {
	entries, err := a.db.GetSummary(ctx)
	if err != nil {
		return nil, err
	}
	return &users.Summary{Entries: entries}, nil
}
