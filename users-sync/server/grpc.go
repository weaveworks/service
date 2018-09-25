package server

import (
	"context"
	"github.com/weaveworks/service/users-sync/attrsync"

	"github.com/weaveworks/common/logging"

	"github.com/weaveworks/service/users-sync/api"
	"github.com/weaveworks/service/users-sync/cleaner"
)

type usersSyncServer struct {
	attributeSyncer *attrsync.AttributeSyncer
	cleaner         *cleaner.OrgCleaner
	log             logging.Interface
}

// New returns a new UsersSyncServer
func New(log logging.Interface) api.UsersSyncServer {
	return &usersSyncServer{log: log}
}

func (u *usersSyncServer) EnqueueUsersSync(ctx context.Context, req *api.EnqueueUsersSyncRequest) (*api.EnqueueUsersSyncResponse, error) {
	err := u.attributeSyncer.EnqueueUsersSync(ctx, req.UserIDs)
	return &api.EnqueueUsersSyncResponse{}, err
}

func (u *usersSyncServer) EnqueueOrgsSync(ctx context.Context, req *api.EnqueueOrgsSyncRequest) (*api.EnqueueOrgsSyncResponse, error) {
	err := u.attributeSyncer.EnqueueOrgsSync(ctx, req.OrgExternalIDs)
	return &api.EnqueueOrgsSyncResponse{}, err
}

func (u *usersSyncServer) EnqueueOrgDeletedSync(ctx context.Context, req *api.EnqueueOrgDeletedSyncRequest) (*api.EnqueueOrgDeletedSyncResponse, error) {
	u.cleaner.Trigger()
	err := u.attributeSyncer.EnqueueOrgsSync(ctx, []string{req.OrgExternalID})
	return &api.EnqueueOrgDeletedSyncResponse{}, err
}
