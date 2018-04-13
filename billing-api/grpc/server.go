package grpc

import (
	"github.com/weaveworks/service/billing-api/db"
	commongrpc "github.com/weaveworks/service/common/billing/grpc"
	"golang.org/x/net/context"
)

// Server implements gRPC's BillingServer interface.
type Server struct {
	DB db.DB
}

// FindBillingAccountByTeamID returns the billing account for the specified team.
func (s Server) FindBillingAccountByTeamID(ctx context.Context, req *commongrpc.BillingAccountByTeamIDRequest) (*commongrpc.BillingAccount, error) {
	account, err := s.DB.FindBillingAccountByTeamID(ctx, req.TeamID)
	if err != nil {
		return nil, err
	}
	return account, nil
}
