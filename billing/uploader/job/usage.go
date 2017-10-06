package job

import (
	"context"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/pkg/errors"

	"github.com/weaveworks/common/instrument"
	"github.com/weaveworks/common/logging"
	"github.com/weaveworks/common/user"
	"github.com/weaveworks/service/billing/db"
	"github.com/weaveworks/service/billing/util"
	timeutil "github.com/weaveworks/service/billing/util/time"
	"github.com/weaveworks/service/billing/zuora"
	"github.com/weaveworks/service/users"
)

var (
	instancesCount = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "billing",
		Subsystem: "uploader",
		Name:      "billable_instances",
		Help:      "Number of billable instances in latest upload",
	}, []string{"status"})
	recordsCount = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "billing",
		Subsystem: "uploader",
		Name:      "records",
		Help:      "Number of aggregated records in latest upload",
	}, []string{"status", "amount_type"})
	amountsCount = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "billing",
		Subsystem: "uploader",
		Name:      "amounts",
		Help:      "Sum of aggregated values in latest upload",
	}, []string{"status", "amount_type"})
)

func init() {
	prometheus.MustRegister(instancesCount)
	prometheus.MustRegister(recordsCount)
	prometheus.MustRegister(amountsCount)
}

// UsageUpload sends aggregates to Zuora.
type UsageUpload struct {
	db        db.DB
	users     users.UsersClient
	zuora     zuora.Client
	collector *instrument.JobCollector
}

// NewUsageUpload instantiates UsageUpload.
func NewUsageUpload(db db.DB, users users.UsersClient, zuora zuora.Client, collector *instrument.JobCollector) *UsageUpload {
	return &UsageUpload{
		db:        db,
		users:     users,
		zuora:     zuora,
		collector: collector,
	}
}

// Run starts the job and logs errors.
func (j *UsageUpload) Run() {
	if err := j.Do(); err != nil {
		log.Errorf("Error running upload job %v", err)
	}
}

// Do starts the job and returns an error if it fails.
func (j *UsageUpload) Do() error {
	return instrument.CollectedRequest(context.Background(), "UsageUpload.Do", j.collector, nil, func(ctx context.Context) error {
		logger := logging.With(ctx)

		now := time.Now().UTC()
		through := now.Truncate(24 * time.Hour)
		// we go back at most one week
		earliest := through.Add(-7 * 24 * time.Hour)

		// Look up the billing-enabled instances where the trial has expired.
		resp, err := j.users.GetBillableOrganizations(ctx, &users.GetBillableOrganizationsRequest{
			Now: through,
		})
		if err != nil {
			logger.Error("Failed getting organizations", err)
			return err
		}

		// Get upper bound of previous upload run
		largestAggregateID, err := j.db.GetUsageUploadLargestAggregateID(ctx)
		if err != nil {
			logger.Errorf("Failed retrieving latest upload: %v", err)
			return err
		}

		logger.Infof("Uploading usage between aggregate_id>%d and bucket_start<%v", largestAggregateID, through)

		var instanceIDs []string
		maxAggregateID := 0
		recordCountsByType := map[string]int64{}
		totalSums := map[string]int64{}
		r := zuora.NewReport(j.zuora.GetConfig())
		for _, org := range resp.Organizations {
			orgCtx := user.InjectOrgID(ctx, org.ID)
			orgLogger := logging.With(orgCtx)
			// We never upload anything before the trial expired
			orgFrom := timeutil.MaxTime(earliest, org.TrialExpiresAt)

			account, aggs := j.checkOrganization(orgCtx, orgLogger, orgFrom, through, &org, largestAggregateID)
			if len(aggs) == 0 {
				continue
			}

			for _, agg := range aggs {
				if agg.ID > maxAggregateID {
					maxAggregateID = agg.ID
				}
				recordCountsByType[agg.AmountType]++
				totalSums[agg.AmountType] += agg.AmountValue
			}

			subscriptionNumber := account.Subscription.SubscriptionNumber
			chargeNumber := account.Subscription.ChargeNumber
			orgReport, err := util.ReportFromAggregates(j.zuora.GetConfig(), aggs, account.PaymentProviderID, orgFrom, through, subscriptionNumber, chargeNumber, zuora.BillCycleDay)
			if err != nil {
				orgLogger.Errorf("Failed to create report: %v", err)
				continue
			}
			r = r.ConcatEntries(orgReport)

			// Keep track of instances for logging purposes
			instanceIDs = append(instanceIDs, org.ID)
		}

		logger.Infof("Found %d billable instances", len(instanceIDs))

		err = j.uploadToZuora(ctx, r, maxAggregateID)

		// Update metrics
		status := "success"
		if err != nil {
			status = "error"
		}
		instancesCount.WithLabelValues(status).Set(float64(len(instanceIDs)))
		for amountType, records := range recordCountsByType {
			recordsCount.WithLabelValues(status, amountType).Set(float64(records))
		}
		for amountType, amountValue := range totalSums {
			amountsCount.WithLabelValues(status, amountType).Set(float64(amountValue))
		}

		return err
	})
}

// checkOrganization verifies whether an organization's usage should be uploaded.
func (j *UsageUpload) checkOrganization(ctx context.Context, logger *log.Entry, from, through time.Time, org *users.Organization, fromID int) (*zuora.Account, []db.Aggregate) {
	// Skip if their trial hasn't expired by the end of this period.
	// GetBillableOrganizations really shouldn't include any such
	// trials, but it's good to double-check.
	if org.InTrialPeriod(through) {
		logger.Warn("Organization returned as 'billable' but trial still ongoing")
		return nil, nil
	}

	// Check if they have a zuora account
	// TODO: move this check to GetBillableOrganizations()
	account, err := j.zuora.GetAccount(ctx, org.ExternalID)
	if err == zuora.ErrNotFound {
		logger.Infof("Instance %v has no Zuora account (delinquent)", org.ExternalID)
		return nil, nil
	}
	if err != nil {
		logger.Errorf("Failing to bill instance %v due to Zuora error: %v", org.ExternalID, err)
		return nil, nil
	}
	if account.PaymentProviderID == "" {
		logger.Infof("Instance %v has no Zuora payment provider", org.ExternalID)
		return nil, nil
	}

	if org.ID == "" {
		logger.Errorf("Internal Instance ID is missing for %v", org.ExternalID)
		return nil, nil
	}

	aggs, err := j.db.GetAggregatesAfter(ctx, org.ID, from, through, fromID)
	if err != nil {
		logger.Errorf("Error querying aggregates database: %v", err)
		return nil, nil
	}
	logger.Infof("Found %d aggregates for %v", len(aggs), org.ExternalID)
	if len(aggs) == 0 {
		return nil, nil
	}

	return account, aggs
}

// uploadToZuora sends the usage listed in the Zuora report. It also records
// when we uploaded to Zuora.
func (j *UsageUpload) uploadToZuora(ctx context.Context, r *zuora.Report, maxAggregateID int) error {
	if r.Size() == 0 {
		return nil
	}

	reader, err := r.ToZuoraFormat()
	if err != nil {
		return err
	}
	uploadID, err := j.db.InsertUsageUpload(ctx, maxAggregateID)
	if err != nil {
		return err
	}
	if _, err = j.zuora.UploadUsage(ctx, reader); err != nil {
		// Delete upload record because we failed, so our next run will picks these aggregates up again.
		if e := j.db.DeleteUsageUpload(ctx, uploadID); e != nil {
			// We couldn't delete the record of uploading usage and therefore will not retry in another run.
			// Manual intervention is required.
			return errors.Wrapf(e, "cannot delete usage_uploads.id==%v (with aggregates_id==%v) after Zuora upload failed, you *must* delete this record manually before the next run", uploadID, maxAggregateID)
		}
		return err
	}

	return nil
}
