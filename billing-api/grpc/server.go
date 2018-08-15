package grpc

import (
	"context"
	log "github.com/sirupsen/logrus"
	"time"

	"github.com/weaveworks/service/billing-api/api"
	"github.com/weaveworks/service/billing-api/db"
	"github.com/weaveworks/service/billing-api/trial"
	commongrpc "github.com/weaveworks/service/common/billing/grpc"
	"github.com/weaveworks/service/common/zuora"
	"github.com/weaveworks/service/users"
)

// Server implements gRPC's BillingServer interface.
type Server struct {
	DB    db.DB
	Users users.UsersClient
	Zuora zuora.Client
}

// FindBillingAccountByTeamID returns the billing account for the specified team.
func (s Server) FindBillingAccountByTeamID(ctx context.Context, req *commongrpc.BillingAccountByTeamIDRequest) (*commongrpc.BillingAccount, error) {
	log.WithField("teamID", req.TeamID).Infof("finding billing account")
	account, err := s.DB.FindBillingAccountByTeamID(ctx, req.TeamID)
	if err != nil {
		return nil, err
	}
	return account, nil
}

// GetInstanceBillingStatus returns the billing status for an instance
func (s Server) GetInstanceBillingStatus(ctx context.Context, req *commongrpc.InstanceBillingStatusRequest) (*commongrpc.InstanceBillingStatusResponse, error) {
	resp, err := s.Users.GetOrganization(ctx, &users.GetOrganizationRequest{
		ID: &users.GetOrganizationRequest_InternalID{InternalID: req.InternalID},
	})
	if err != nil {
		return nil, err
	}
	org := resp.Organization
	now := time.Now().UTC()
	zuoraAcct, err := s.Zuora.GetAccount(ctx, org.ZuoraAccountNumber)
	trial := trial.Info(org.TrialExpiresAt, org.CreatedAt, now)
	status, _, _ := api.GetBillingStatus(ctx, trial, zuoraAcct)
	return &commongrpc.InstanceBillingStatusResponse{BillingStatus: status}, nil
}
