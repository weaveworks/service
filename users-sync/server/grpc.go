package server

import (
	"golang.org/x/net/context"

	"github.com/weaveworks/common/logging"

	"github.com/weaveworks/service/users-sync/api"
	"github.com/weaveworks/service/users-sync/cleaner"
	"github.com/weaveworks/service/users-sync/weeklyreporter"
)

type usersSyncServer struct {
	weeklyReporter *weeklyreporter.WeeklyReporter
	cleaner        *cleaner.OrgCleaner
	log            logging.Interface
}

// New returns a new UsersSyncServer
func New(log logging.Interface, cleaner *cleaner.OrgCleaner, weeklyReporter *weeklyreporter.WeeklyReporter) api.UsersSyncServer {
	return &usersSyncServer{
		weeklyReporter,
		cleaner,
		log,
	}
}

func (u *usersSyncServer) EnqueueOrgDeletedSync(ctx context.Context, req *api.EnqueueOrgDeletedSyncRequest) (*api.EnqueueOrgDeletedSyncResponse, error) {
	u.cleaner.Trigger()
	return &api.EnqueueOrgDeletedSyncResponse{}, nil
}

func (u *usersSyncServer) EnforceWeeklyReporterJob(ctx context.Context, req *api.EnforceWeeklyReporterJobRequest) (*api.EnforceWeeklyReporterJobResponse, error) {
	err := u.weeklyReporter.Job.Do()
	return &api.EnforceWeeklyReporterJobResponse{}, err
}
