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

// ReporterJob for weekly reporting.
type ReporterJob struct {
	log   logging.Interface
	users users.UsersClient
}

// Run starts the job and logs errors.
func (j *ReporterJob) Run() {
	if err := j.Do(); err != nil {
		j.log.Errorf("Error running job: %v", err)
	}
}

// Do starts the job and returns an error if it fails.
func (j *ReporterJob) Do() error {
	return j.sendOutWeeklyReportForAllInstances(context.Background())
}

func (j *ReporterJob) sendOutWeeklyReportForAllInstances(ctx context.Context) error {
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

// WeeklyReporter consists of the scheduled job + cron scheduler.
type WeeklyReporter struct {
	scheduler *cron.Cron
	Job       *ReporterJob
}

// New schedules a new weekly reporter that runs on a regular basis.
func New(log logging.Interface, users users.UsersClient) *WeeklyReporter {
	job := ReporterJob{
		log:   log,
		users: users,
	}

	cronScheduler := cron.New()
	cronScheduler.AddJob(cronSchedule, &job)

	return &WeeklyReporter{
		scheduler: cronScheduler,
		Job:       &job,
	}
}

// Start starts the cron scheduler.
func (w *WeeklyReporter) Start() {
	w.scheduler.Start()
}

// Stop stops the cron scheduler.
func (w *WeeklyReporter) Stop() {
	w.scheduler.Stop()
}
