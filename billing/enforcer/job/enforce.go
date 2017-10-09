package job

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"

	"flag"
	"github.com/weaveworks/common/instrument"
	"github.com/weaveworks/common/logging"
	"github.com/weaveworks/service/users"
)

var trialNotifiedCount = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Namespace: "billing",
	Subsystem: "enforcer",
	Name:      "trial_notified",
	Help:      "Number of organizations notified",
}, []string{"type", "status"})

func init() {
	prometheus.MustRegister(trialNotifiedCount)
}

// Config provides settings for this job.
type Config struct {
	NotifyPendingExpiryPeriod time.Duration
}

// RegisterFlags registers configuration variables.
func (cfg *Config) RegisterFlags(f *flag.FlagSet) {
	f.DurationVar(&cfg.NotifyPendingExpiryPeriod,
		"trial.notify-pending-expiry-period", 3*24*time.Hour,
		"Duration before trial expiry when we send the notification")
}

// Enforce job sends notification emails.
type Enforce struct {
	users     users.UsersClient
	cfg       Config
	collector *instrument.JobCollector
}

// NewEnforce instantiates Enforce.
func NewEnforce(client users.UsersClient, cfg Config, collector *instrument.JobCollector) *Enforce {
	return &Enforce{
		users:     client,
		cfg:       cfg,
		collector: collector,
	}
}

// Run starts the job and logs errors.
func (j *Enforce) Run() {
	if err := j.Do(); err != nil {
		log.Errorf("Error running job %v", err)
	}
}

// Do starts the job and returns an error if it fails.
func (j *Enforce) Do() error {
	return instrument.CollectedRequest(context.Background(), "Enforce.Do", j.collector, nil, func(ctx context.Context) error {
		now := time.Now().UTC()
		terr := j.NotifyTrialOrganizations(ctx, now)
		derr := j.NotifyDelinquentOrganizations(ctx, now)

		if terr != nil && derr != nil {
			return fmt.Errorf("%v\n%v", terr, derr)
		}
		if terr != nil {
			return terr
		}
		if derr != nil {
			return derr
		}
		return nil
	})
}

// NotifyTrialOrganizations sends emails to organizations for pending trial expiry.
func (j *Enforce) NotifyTrialOrganizations(ctx context.Context, now time.Time) error {
	logger := logging.With(ctx)

	resp, err := j.users.GetTrialOrganizations(ctx, &users.GetTrialOrganizationsRequest{
		Now: now,
	})
	if err != nil {
		return errors.Wrap(err, "failed getting trial organizations")
	}
	logger.Infof("Received %d trial organizations", len(resp.Organizations))

	ok := 0
	fail := 0
	for _, org := range resp.Organizations {
		// TODO: move filters to GetTrialOrganizationsRequest

		// Have we already notified?
		if org.TrialPendingExpiryNotifiedAt != nil {
			continue
		}
		// Are we within notification range of expiration?
		expiresIn := org.TrialExpiresAt.Sub(now)
		if expiresIn > j.cfg.NotifyPendingExpiryPeriod {
			continue
		}
		// No payment method added?
		if org.ZuoraAccountNumber != "" {
			continue
		}

		_, err := j.users.NotifyTrialPendingExpiry(ctx, &users.NotifyTrialPendingExpiryRequest{
			ExternalID: org.ExternalID,
		})
		if err == nil {
			ok++
		} else {
			logger.Errorf("Failed notifying trial organization %s: %v", org.ExternalID, err)
			fail++
		}
	}
	trialNotifiedCount.WithLabelValues("trial_pending_expiry", "success").Set(float64(ok))
	trialNotifiedCount.WithLabelValues("trial_pending_expiry", "error").Set(float64(fail))

	return nil
}

// NotifyDelinquentOrganizations sends emails to organizations whose trial has expired.
func (j *Enforce) NotifyDelinquentOrganizations(ctx context.Context, now time.Time) error {
	logger := logging.With(ctx)

	resp, err := j.users.GetDelinquentOrganizations(ctx, &users.GetDelinquentOrganizationsRequest{
		Now: now,
	})
	if err != nil {
		return errors.Wrap(err, "failed getting delinquent organizations")
	}
	logger.Infof("Received %d delinquent organizations", len(resp.Organizations))

	ok := 0
	fail := 0
	for _, org := range resp.Organizations {
		// TODO: move filters to GetDelinquentOrganizationsRequest

		// Have we already notified?
		if org.TrialExpiredNotifiedAt != nil {
			continue
		}

		_, err := j.users.NotifyTrialExpired(ctx, &users.NotifyTrialExpiredRequest{
			ExternalID: org.ExternalID,
		})
		if err == nil {
			ok++
		} else {
			logger.Errorf("Failed notifying delinquent organization %s: %v", org.ExternalID, err)
			fail++
		}
	}
	trialNotifiedCount.WithLabelValues("trial_expired", "success").Set(float64(ok))
	trialNotifiedCount.WithLabelValues("trial_expired", "error").Set(float64(fail))

	return nil
}
