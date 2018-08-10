package client

import (
	"errors"

	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"github.com/weaveworks/service/users"
)

// MockAuthClient is a mock users.AuthServiceClient that can be used in testing
type MockAuthClient struct{}

var _ users.AuthServiceClient = &MockAuthClient{}

// AuthUserForOrg authenticates a cookie for access to an org by external ID.
func (MockAuthClient) AuthUserForOrg(ctx context.Context, in *users.LookupOrgRequest, opts ...grpc.CallOption) (*users.LookupOrgResponse, error) {
	return &users.LookupOrgResponse{
		OrganizationID: "mockID",
		UserID:         "mockUserID",
	}, nil
}

// AuthTokenForOrg authenticates a token for access to an org.
func (MockAuthClient) AuthTokenForOrg(ctx context.Context, in *users.LookupUsingTokenRequest, opts ...grpc.CallOption) (*users.LookupUsingTokenResponse, error) {
	return &users.LookupUsingTokenResponse{
		OrganizationID: "mockID",
	}, nil
}

// AuthUserForAdmin authenticates a cookie for admin access.
func (MockAuthClient) AuthUserForAdmin(ctx context.Context, in *users.LookupAdminRequest, opts ...grpc.CallOption) (*users.LookupAdminResponse, error) {
	return &users.LookupAdminResponse{
		AdminID: "mockUserID",
	}, nil
}

// AuthUser authenticates a cookie.
func (MockAuthClient) AuthUser(ctx context.Context, in *users.LookupUserRequest, opts ...grpc.CallOption) (*users.LookupUserResponse, error) {
	return &users.LookupUserResponse{
		UserID: "mockUserID",
	}, nil
}

// AuthWebhookSecretForOrg gets the webhook given the external org ID and the secret ID of the webhook.
func (MockAuthClient) AuthWebhookSecretForOrg(ctx context.Context, in *users.LookupOrganizationWebhookUsingSecretIDRequest, opts ...grpc.CallOption) (*users.LookupOrganizationWebhookUsingSecretIDResponse, error) {
	return &users.LookupOrganizationWebhookUsingSecretIDResponse{}, nil
}

// MockClient is a mock usersClient that can be used in testing
type MockClient struct{}

var _ users.UsersClient = &MockClient{}

// GetOrganizations gets the organizations for a user
func (MockClient) GetOrganizations(ctx context.Context, in *users.GetOrganizationsRequest, opts ...grpc.CallOption) (*users.GetOrganizationsResponse, error) {
	return &users.GetOrganizationsResponse{}, nil
}

// GetBillableOrganizations returns all of the organizations that are past
// their trial period and have billing enabled. Currently knows nothing
// about payment status, so will include organizations that are well past
// their trial period but haven't provided credit card details.
func (MockClient) GetBillableOrganizations(ctx context.Context, in *users.GetBillableOrganizationsRequest, opts ...grpc.CallOption) (*users.GetBillableOrganizationsResponse, error) {
	return &users.GetBillableOrganizationsResponse{}, nil
}

// GetTrialOrganizations returns all organizations that are currently in their
// trial period.
func (MockClient) GetTrialOrganizations(ctx context.Context, in *users.GetTrialOrganizationsRequest, opts ...grpc.CallOption) (*users.GetTrialOrganizationsResponse, error) {
	return &users.GetTrialOrganizationsResponse{}, nil
}

// GetDelinquentOrganizations returns all organizations that are beyond their
// trial period and haven't yet supplied any payment method. We determine this
// by means of having a Zuora account.
func (MockClient) GetDelinquentOrganizations(ctx context.Context, in *users.GetDelinquentOrganizationsRequest, opts ...grpc.CallOption) (*users.GetDelinquentOrganizationsResponse, error) {
	return &users.GetDelinquentOrganizationsResponse{}, nil
}

// GetOrganization gets an organization by its internal or external ID
func (MockClient) GetOrganization(ctx context.Context, in *users.GetOrganizationRequest, opts ...grpc.CallOption) (*users.GetOrganizationResponse, error) {
	return &users.GetOrganizationResponse{}, nil
}

// GetUser returns details for a user
func (MockClient) GetUser(ctx context.Context, in *users.GetUserRequest, opts ...grpc.CallOption) (*users.GetUserResponse, error) {
	return &users.GetUserResponse{
		User: users.User{
			ID:    "1",
			Email: "mock-user@example.org",
		},
	}, nil
}

// SetOrganizationFlag sets an org flag
func (MockClient) SetOrganizationFlag(ctx context.Context, in *users.SetOrganizationFlagRequest, opts ...grpc.CallOption) (*users.SetOrganizationFlagResponse, error) {
	return &users.SetOrganizationFlagResponse{}, nil
}

// SetOrganizationZuoraAccount updates zuora account information. It should only
// be called when changed which denotes that an account has been created. If you
// omit `ZuoraAccountCreatedAt` it will be automatically updated to now.
func (MockClient) SetOrganizationZuoraAccount(ctx context.Context, in *users.SetOrganizationZuoraAccountRequest, opts ...grpc.CallOption) (*users.SetOrganizationZuoraAccountResponse, error) {
	return &users.SetOrganizationZuoraAccountResponse{}, nil
}

// NotifyTrialPendingExpiry sends a "Trial expiring soon" notification
// to this user and records the date sent.
func (MockClient) NotifyTrialPendingExpiry(ctx context.Context, in *users.NotifyTrialPendingExpiryRequest, opts ...grpc.CallOption) (*users.NotifyTrialPendingExpiryResponse, error) {
	return &users.NotifyTrialPendingExpiryResponse{}, nil
}

// NotifyTrialExpired sends a "Trial expired" notification to this user
// and records the date sent.
func (MockClient) NotifyTrialExpired(ctx context.Context, in *users.NotifyTrialExpiredRequest, opts ...grpc.CallOption) (*users.NotifyTrialExpiredResponse, error) {
	return &users.NotifyTrialExpiredResponse{}, nil
}

// NotifyRefuseDataUpload sends a "data upload blocked" notification to the members
// of this organization.
func (MockClient) NotifyRefuseDataUpload(ctx context.Context, in *users.NotifyRefuseDataUploadRequest, opts ...grpc.CallOption) (*users.NotifyRefuseDataUploadResponse, error) {
	return &users.NotifyRefuseDataUploadResponse{}, nil
}

// GetGCP returns the Google Cloud Platform entry.
func (MockClient) GetGCP(ctx context.Context, in *users.GetGCPRequest, opts ...grpc.CallOption) (*users.GetGCPResponse, error) {
	return &users.GetGCPResponse{}, nil
}

// UpdateGCP updates the Google Cloud Platform entry.
func (MockClient) UpdateGCP(ctx context.Context, in *users.UpdateGCPRequest, opts ...grpc.CallOption) (*users.UpdateGCPResponse, error) {
	return &users.UpdateGCPResponse{}, nil
}

// GetSummary exports a summary of the DB.
func (MockClient) GetSummary(ctx context.Context, in *users.Empty, opts ...grpc.CallOption) (*users.Summary, error) {
	return &users.Summary{}, nil
}

// SetOrganizationWebhookFirstSeenAt sets the FirstSeenAt field on the webhook to the current time
func (MockClient) SetOrganizationWebhookFirstSeenAt(ctx context.Context, in *users.SetOrganizationWebhookFirstSeenAtRequest, opts ...grpc.CallOption) (*users.SetOrganizationWebhookFirstSeenAtResponse, error) {
	return &users.SetOrganizationWebhookFirstSeenAtResponse{}, nil
}

var errLegacy = errors.New("Legacy mocks methods, to be removed once we remove these from the proto file")

// LookupOrg authenticates a cookie for access to an org by external ID.
func (MockClient) LookupOrg(ctx context.Context, in *users.LookupOrgRequest, opts ...grpc.CallOption) (*users.LookupOrgResponse, error) {
	return nil, errLegacy
}

// LookupUsingToken authenticates a token for access to an org.
func (MockClient) LookupUsingToken(ctx context.Context, in *users.LookupUsingTokenRequest, opts ...grpc.CallOption) (*users.LookupUsingTokenResponse, error) {
	return nil, errLegacy
}

// LookupAdmin authenticates a cookie for admin access.
func (MockClient) LookupAdmin(ctx context.Context, in *users.LookupAdminRequest, opts ...grpc.CallOption) (*users.LookupAdminResponse, error) {
	return nil, errLegacy
}

// LookupUser authenticates a cookie.
func (MockClient) LookupUser(ctx context.Context, in *users.LookupUserRequest, opts ...grpc.CallOption) (*users.LookupUserResponse, error) {
	return nil, errLegacy
}

// LookupOrganizationWebhookUsingSecretID gets the webhook given the external org ID and the secret ID of the webhook.
func (MockClient) LookupOrganizationWebhookUsingSecretID(ctx context.Context, in *users.LookupOrganizationWebhookUsingSecretIDRequest, opts ...grpc.CallOption) (*users.LookupOrganizationWebhookUsingSecretIDResponse, error) {
	return nil, errLegacy
}
