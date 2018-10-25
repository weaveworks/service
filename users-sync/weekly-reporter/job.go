package weeklyreporter

import (
	"context"

	"github.com/robfig/cron"
	"github.com/weaveworks/common/logging"
	"github.com/weaveworks/service/users"
)

const (
	cronSchedule = "0 0 8 * * 1" // Every Monday at 8:00 AM (UTC).
)

// Job for weekly reporting.
type Job struct {
	log   logging.Interface
	users users.UsersClient
}

// New schedules a new weekly reporting job to be run on a regular basis.
func New(log logging.Interface, users users.UsersClient) *cron.Cron {
	log.Infoln("Run weekly reporter")

	cronScheduler := cron.New()
	cronScheduler.AddJob(cronSchedule, &Job{
		log:   log,
		users: users,
	})

	return cronScheduler
}

// Run starts the job and logs errors.
func (j *Job) Run() {
	if err := j.Do(); err != nil {
		j.log.Errorf("Error running job: %v", err)
	}
}

// Do starts the job and returns an error if it fails.
func (j *Job) Do() error {
	return j.sendOutWeeklyReportForAllInstances(context.Background())
}

func (j *Job) sendOutWeeklyReportForAllInstances(ctx context.Context) error {
	resp, err := j.users.GetOrganizationsReadyForWeeklyReport(ctx, &users.GetOrganizationsReadyForWeeklyReportRequest{})
	if err != nil {
		j.log.Errorf("WeeklyReports: error fetching the instances for reports: %v", err)
		return err
	}
	j.log.Infof("WeeklyReports: sending out emails to members of %d instances", len(resp.Organizations))

	for _, organization := range resp.Organizations {
		request := users.SendOutWeeklyReportRequest{ExternalID: organization.ExternalID}
		if _, err := j.users.SendOutWeeklyReport(ctx, &request); err != nil {
			// Only log the error and move to the next instance if sending out weekly report fails.
			j.log.Errorf("WeeklyReports: error sending report for '%s': %v", organization.ExternalID, err)
		}
	}

	return nil
}
