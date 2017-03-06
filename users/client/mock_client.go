package client

import (
	"context"

	"google.golang.org/grpc"

	"github.com/weaveworks/service/users"
)

type mockClient struct{}

// LookupOrg authenticates a cookie for access to an org by extenal ID.
func (mockClient) LookupOrg(ctx context.Context, in *users.LookupOrgRequest, opts ...grpc.CallOption) (*users.LookupOrgResponse, error) {
	return &users.LookupOrgResponse{
		OrganizationID: "mockID",
		UserID:         "mockUserID",
	}, nil
}

// LookupUsingToken authenticates a token for access to an org.
func (mockClient) LookupUsingToken(ctx context.Context, in *users.LookupUsingTokenRequest, opts ...grpc.CallOption) (*users.LookupUsingTokenResponse, error) {
	return &users.LookupUsingTokenResponse{
		OrganizationID: "mockID",
	}, nil
}

// LookupAdmin authenticates a cookie for admin access.
func (mockClient) LookupAdmin(ctx context.Context, in *users.LookupAdminRequest, opts ...grpc.CallOption) (*users.LookupAdminResponse, error) {
	return &users.LookupAdminResponse{
		AdminID: "mockUserID",
	}, nil
}

// LookupUser authenticates a cookie.
func (mockClient) LookupUser(ctx context.Context, in *users.LookupUserRequest, opts ...grpc.CallOption) (*users.LookupUserResponse, error) {
	return &users.LookupUserResponse{
		UserID: "mockUserID",
	}, nil
}
