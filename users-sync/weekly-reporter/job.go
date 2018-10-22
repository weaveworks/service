package weeklyreporter

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/weaveworks/common/instrument"
	"github.com/weaveworks/common/logging"
	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/db"
	"github.com/weaveworks/service/users/db/filter"
)

// Job blu.
type Job struct {
	db        db.DB
	users     users.UsersClient
	log       logging.Interface
	collector *instrument.JobCollector
}

// New instantiates Enforce.
func New(db db.DB, users users.UsersClient, log logging.Interface, collector *instrument.JobCollector) *Job {
	return &Job{
		db:        db,
		users:     users,
		log:       log,
		collector: collector,
	}
}

// Run starts the job and logs errors.
func (j *Job) Run() {
	if err := j.Do(); err != nil {
		log.Errorf("Error running job: %v", err)
	}
}

// Do starts the job and returns an error if it fails.
func (j *Job) Do() error {
	organizations, err := j.db.ListOrganizations(context.Background(), filter.All, 0)
	if err != nil {
		j.log.Errorf("%v\n", err)
		return err
	}

	j.log.Infof("Sending out weekly report emails to users of %d instances", len(organizations))

	almostAWeekAgo := time.Now().AddDate(0, 0, 6)

	for _, organization := range organizations {
		if organization.LastSentWeeklyReportAt.Before(almostAWeekAgo) {
			j.log.Infof("Sending weekly report to %s", organization.ExternalID)
			_, err := j.users.SendOutWeeklyReport(context.Background(), &users.SendOutWeeklyReportRequest{
				ExternalID: organization.ExternalID,
			})
			if err != nil {
				j.log.Errorln(err)
			}
		}
	}

	return nil
}
