package weeklyreporter

import (
	"flag"

	"github.com/robfig/cron"
	"github.com/weaveworks/common/instrument"
	"github.com/weaveworks/common/logging"
	"github.com/weaveworks/service/common/users"
	"github.com/weaveworks/service/users/db"
)

var jobCollector = instrument.NewJobCollector("weeklyreports")

func init() {
	jobCollector.Register()
}

const (
	cronSchedule = "0 8 * * 1" // Every Monday at 8:00 AM UTC.
)

// NewJob runs org cleaner
func NewJob(log logging.Interface, db db.DB) *cron.Cron {
	log.Infoln("Run weekly reporter")

	var usersConfig users.Config
	usersConfig.RegisterFlags(flag.CommandLine)
	users, err := users.NewClient(usersConfig)
	if err != nil {
		log.Errorf("error initialising users client: %v", err)
	}

	c := cron.New()
	job := New(db, users, log, jobCollector)
	c.AddJob(cronSchedule, job)

	job.Do()

	return c
}
