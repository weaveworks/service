package weeklyreporter

import (
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
	// usersServiceURL = "users.default:4772" // URL to connect to users service.
	usersServiceURL = "users.default.svc.cluster.local:4772" // URL to connect to users service.
)

// NewJob runs org cleaner
func NewJob(log logging.Interface, db db.DB) *cron.Cron {
	log.Infoln("Run weekly reporter")

	users, err := users.NewClient(users.Config{HostPort: usersServiceURL})
	if err != nil {
		log.Errorf("error initialising users client: %v", err)
	}

	c := cron.New()
	job := New(db, users, log, jobCollector)
	c.AddJob(cronSchedule, job)

	job.Do()

	return c
}
