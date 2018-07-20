package job

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"

	"flag"
	"github.com/weaveworks/common/instrument"
	"github.com/weaveworks/common/logging"
	"github.com/weaveworks/common/user"
	"github.com/weaveworks/service/billing-api/trial"
	"github.com/weaveworks/service/common/orgs"
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
		log.Errorf("Error running job: %v", err)
	}
}

// Do starts the job and returns an error if it fails.
func (j *Enforce) Do() error {
	return instrument.CollectedRequest(context.Background(), "Enforce.Do", j.collector, nil, func(ctx context.Context) error {
		now := time.Now().UTC()
		var errs []string
		for _, call := range []func(context.Context, time.Time) error{
			j.NotifyTrialOrganizations,
			j.ProcessDelinquentOrganizations,
		} {
			if err := call(ctx, now); err != nil {
				errs = append(errs, err.Error())
			}
		}
		if len(errs) > 0 {
			return fmt.Errorf("%v", strings.Join(errs, " / "))
		}
		return nil
	})
}

// NotifyTrialOrganizations sends emails to organizations for pending trial expiry.
func (j *Enforce) NotifyTrialOrganizations(ctx context.Context, now time.Time) error {
	logger := user.LogWith(ctx, logging.Global())

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

		if !org.IsOnboarded() {
			continue
		}

		// Have we already notified?
		if org.TrialPendingExpiryNotifiedAt != nil {
			continue
		}
		// Are we within notification range of expiration?
		expiresIn := org.TrialExpiresAt.Sub(now)
		if expiresIn > j.cfg.NotifyPendingExpiryPeriod {
			continue
		}

		// Trial already expired
		if expiresIn <= 0 {
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
			logger.Infof("Notified trial pending expiry for organization: %s", org.ExternalID)
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

// ProcessDelinquentOrganizations sends emails to organizations whose trial has expired.
// It also makes sure their RefuseDataAccess flag is set and if delinquent for more than
// 15 days, RefuseDataUpload is set.
func (j *Enforce) ProcessDelinquentOrganizations(ctx context.Context, now time.Time) error {
	logger := user.LogWith(ctx, logging.Global())

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

		if !org.IsOnboarded() {
			continue
		}

		// Failure in any of these flag updates should not interfere with the notification
		// email. It will just pick it up on the next run. We do log an error.
		j.refuseDataAccess(ctx, org, now)

		// Only send notification if the instance actually had any trial and the RefuseDataUpload flag
		// hasn't been set already. If there was no trial, they already received the `trial_expired` email today.
		// No need to flood.
		if j.refuseDataUpload(ctx, org, now) && trial.Length(org.TrialExpiresAt, org.CreatedAt) > 0 {
			if _, err := j.users.NotifyRefuseDataUpload(ctx, &users.NotifyRefuseDataUploadRequest{ExternalID: org.ExternalID}); err == nil {
				logger.Infof("Notified data upload refusal for organization: %s", org.ExternalID)
			} else {
				logger.Errorf("Failed notifying data upload refusal for organization %s: %v", org.ExternalID, err)
			}
		}

		// Have we already notified?
		if org.TrialExpiredNotifiedAt != nil {
			continue
		}

		_, err := j.users.NotifyTrialExpired(ctx, &users.NotifyTrialExpiredRequest{
			ExternalID: org.ExternalID,
		})
		if err == nil {
			logger.Infof("Notified trial expired for organization: %s", org.ExternalID)
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

func (j *Enforce) refuseDataAccess(ctx context.Context, org users.Organization, now time.Time) {
	if org.RefuseDataAccess {
		return
	}
	if !orgs.ShouldRefuseDataAccess(org, now) {
		return
	}

	_, err := j.users.SetOrganizationFlag(ctx, &users.SetOrganizationFlagRequest{
		ExternalID: org.ExternalID,
		Flag:       orgs.RefuseDataAccess,
		Value:      true,
	})
	logger := user.LogWith(ctx, logging.Global())
	if err == nil {
		logger.Infof("Refused data access for organization: %s", org.ExternalID)
	} else {
		logger.Errorf("Failed refusing data access for organization %s: %v", org.ExternalID, err)
	}
}

func (j *Enforce) refuseDataUpload(ctx context.Context, org users.Organization, now time.Time) bool {
	if org.RefuseDataUpload {
		return false
	}
	if !orgs.ShouldRefuseDataUpload(org, now) {
		return false
	}
	_, err := j.users.SetOrganizationFlag(ctx, &users.SetOrganizationFlagRequest{
		ExternalID: org.ExternalID,
		Flag:       orgs.RefuseDataUpload,
		Value:      true,
	})

	logger := user.LogWith(ctx, logging.Global())
	if err == nil {
		logger.Infof("Refused data upload for organization: %s", org.ExternalID)
	} else {
		logger.Errorf("Failed refusing data upload for organization %s: %v", org.ExternalID, err)
	}
	return true
}
