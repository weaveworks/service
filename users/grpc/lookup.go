package grpc

import (
	"errors"
	"fmt"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/weaveworks/service/common/featureflag"
	"github.com/weaveworks/service/common/orgs"
	"golang.org/x/net/context"

	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/api"
	"github.com/weaveworks/service/users/db"
	"github.com/weaveworks/service/users/db/filter"
	"github.com/weaveworks/service/users/emailer"
	"github.com/weaveworks/service/users/marketing"
	"github.com/weaveworks/service/users/sessions"
	"github.com/weaveworks/service/users/weeklyreports"
)

// usersServer implements users.UsersServer
type usersServer struct {
	sessions          sessions.Store
	db                db.DB
	emailer           emailer.Emailer
	marketingQueues   marketing.Queues
	forceFeatureFlags []string
}

// New makes a new users.UsersServer
func New(sessions sessions.Store, db db.DB, emailer emailer.Emailer, marketingQueues marketing.Queues, forceFeatureFlags []string) users.UsersServer {
	return &usersServer{
		sessions:          sessions,
		db:                db,
		emailer:           emailer,
		marketingQueues:   marketingQueues,
		forceFeatureFlags: forceFeatureFlags,
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

// LookupOrg authenticates a cookie for access to an org by external ID.
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
	fs := []filter.Filter{}
	if req.Query != "" {
		fs = append(fs, filter.ExternalID(req.Query))
	}
	var organizations []*users.Organization
	var err error
	if req.IncludeDeleted {
		organizations, err = a.db.ListAllOrganizations(ctx, filter.And(fs...), "", uint64(req.PageNumber))
	} else {
		organizations, err = a.db.ListOrganizations(ctx, filter.And(fs...), uint64(req.PageNumber))
	}
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
	members, err := a.db.ListOrganizationUsers(ctx, req.ExternalID, false, true)
	if err != nil {
		return nil, err
	}
	err = a.emailer.TrialPendingExpiryEmail(ctx, members, req.ExternalID, org.Name, org.TrialExpiresAt)
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
	members, err := a.db.ListOrganizationUsers(ctx, req.ExternalID, false, true)
	if err != nil {
		return nil, err
	}
	err = a.emailer.TrialExpiredEmail(ctx, members, req.ExternalID, org.Name)
	if err != nil {
		return nil, err
	}

	// Persist sent date in db
	now := time.Now()
	_, err = a.db.UpdateOrganization(ctx, req.ExternalID, users.OrgWriteView{TrialExpiredNotifiedAt: &now})

	return &users.NotifyTrialExpiredResponse{}, err
}

func (a *usersServer) featureFlagIsForced(flag string) bool {
	for _, f := range a.forceFeatureFlags {
		if f == flag {
			return true
		}
	}
	return false
}

func (a *usersServer) GetOrganizationsReadyForWeeklyReport(ctx context.Context, req *users.GetOrganizationsReadyForWeeklyReportRequest) (*users.GetOrganizationsReadyForWeeklyReportResponse, error) {
	// Get all organizations for which weekly reporting is enabled and that haven't been reported at least for a week.
	endOfSameDayLastWeek := req.Now.UTC().Truncate(24*time.Hour).AddDate(0, 0, -6)
	fs := []filter.Filter{
		filter.LastSentWeeklyReportBefore(endOfSameDayLastWeek),
		filter.SeenPromConnected(true),
	}
	if !a.featureFlagIsForced(featureflag.WeeklyReportable) {
		fs = append(fs, filter.HasFeatureFlag(featureflag.WeeklyReportable))
	}
	organizations, err := a.db.ListOrganizations(
		ctx,
		filter.And(fs...),
		0,
	)
	if err != nil {
		return nil, err
	}

	// Copy all filtered organizations into the response object.
	response := &users.GetOrganizationsReadyForWeeklyReportResponse{}
	for _, org := range organizations {
		response.Organizations = append(response.Organizations, *org)
	}
	return response, nil
}

func (a *usersServer) SendOutWeeklyReport(ctx context.Context, req *users.SendOutWeeklyReportRequest) (*users.SendOutWeeklyReportResponse, error) {
	// Make sure the organization exists
	org, err := a.db.FindOrganizationByID(ctx, req.ExternalID)
	if err != nil {
		return nil, err
	}

	// Generate the weekly report for the organization
	log.Debugf("Generating weekly report for '%s'", org.ExternalID)
	weeklyReport, err := weeklyreports.GenerateReport(org, req.Now)
	if err != nil {
		return nil, err
	}

	// Get all users belonging to the organization
	members, err := a.db.ListOrganizationUsers(ctx, org.ExternalID, false, true)
	if err != nil {
		return nil, err
	}

	if err := a.emailer.WeeklyReportEmail(ctx, members, weeklyReport); err != nil {
		return nil, err
	}

	// Persist weekly report timestamp in the db
	err = a.db.SetLastSentWeeklyReportAt(ctx, req.ExternalID, &req.Now)
	if err != nil {
		return nil, err
	}

	log.Debugf("Sent out weekly report email to %d members of '%s'", len(members), org.ExternalID)
	return &users.SendOutWeeklyReportResponse{}, nil
}

func (a *usersServer) NotifyRefuseDataUpload(ctx context.Context, req *users.NotifyRefuseDataUploadRequest) (*users.NotifyRefuseDataUploadResponse, error) {
	// Make sure the organization exists
	org, err := a.db.FindOrganizationByID(ctx, req.ExternalID)
	if err != nil {
		return nil, err
	}

	// Notify all users
	members, err := a.db.ListOrganizationUsers(ctx, req.ExternalID, false, true)
	if err != nil {
		return nil, err
	}
	err = a.emailer.RefuseDataUploadEmail(ctx, members, req.ExternalID, org.Name)
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

func (a *usersServer) InformOrganizationBillingConfigured(ctx context.Context, req *users.InformOrganizationBillingConfiguredRequest) (*users.Empty, error) {
	org, err := a.db.FindOrganizationByID(ctx, req.ExternalID)
	if err != nil {
		return nil, err
	}

	members, err := a.db.ListOrganizationUsers(ctx, req.ExternalID, false, true)
	if err != nil {
		return nil, err
	}

	for _, member := range members {
		a.marketingQueues.OrganizationBillingConfigured(member.Email, org.ExternalID, org.Name)
	}

	return &users.Empty{}, nil
}

func (a *usersServer) RequireTeamMemberPermissionTo(ctx context.Context, req *users.RequireTeamMemberPermissionToRequest) (*users.Empty, error) {
	teamExternalID := req.GetTeamExternalID()
	if teamExternalID == "" {
		team, err := a.db.FindTeamByInternalID(ctx, req.GetTeamInternalID())
		if err != nil {
			return &users.Empty{}, err
		}
		teamExternalID = team.ExternalID
	}

	return &users.Empty{}, api.RequireTeamMemberPermissionTo(ctx, a.db, req.UserID, teamExternalID, req.PermissionID)
}

func (a *usersServer) RequireOrgMemberPermissionTo(ctx context.Context, req *users.RequireOrgMemberPermissionToRequest) (*users.Empty, error) {
	orgExternalID := req.GetOrgExternalID()
	if orgExternalID == "" {
		org, err := a.db.FindOrganizationByInternalID(ctx, req.GetOrgInternalID())
		if err != nil {
			return &users.Empty{}, err
		}
		orgExternalID = org.ExternalID
	}

	return &users.Empty{}, api.RequireOrgMemberPermissionTo(ctx, a.db, req.UserID, orgExternalID, req.PermissionID)
}
