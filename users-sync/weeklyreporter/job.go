package weeklyreporter

import (
	"context"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/robfig/cron"
	"golang.org/x/time/rate"

	"github.com/weaveworks/common/logging"
	"github.com/weaveworks/service/users"
)

const (
	// Run the job every Monday at 08:00:00 AM (UTC).
	// The first zero stands for seconds - see https://godoc.org/github.com/robfig/cron for more details.
	cronSchedule = "0 0 8 * * 1"
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
	return j.sendOutWeeklyReportForAllInstances(context.Background(), time.Now())
}

func (j *ReporterJob) sendOutWeeklyReportForAllInstances(ctx context.Context, now time.Time) error {
	resp, err := j.users.GetOrganizationsReadyForWeeklyReport(ctx, &users.GetOrganizationsReadyForWeeklyReportRequest{Now: now})
	if err != nil {
		j.log.Errorf("WeeklyReports: error fetching the instances for reports: %v", err)
		return err
	}
	j.log.Infof("WeeklyReports: sending out emails to members of %d instances", len(resp.Organizations))

	limiter := rate.NewLimiter(0.1, 1)

	for _, organization := range resp.Organizations {
		limiter.Wait(ctx)

		span, sendContext := opentracing.StartSpanFromContext(ctx, "SendOutWeeklyReport")
		span.SetTag("organization", organization.ID)

		// Force sampling to debug weaveworks/service-conf#2959
		ext.SamplingPriority.Set(span, 1)

		request := users.SendOutWeeklyReportRequest{Now: now, ExternalID: organization.ExternalID}
		if _, err := j.users.SendOutWeeklyReport(sendContext, &request); err != nil {
			// Only log the error and move to the next instance if sending out weekly report fails.
			j.log.Errorf("WeeklyReports: error sending report for '%s': %v", organization.ExternalID, err)
		}

		span.Finish()
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
