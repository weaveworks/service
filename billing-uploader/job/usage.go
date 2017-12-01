package job

import (
	"context"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"

	"github.com/pkg/errors"

	"github.com/weaveworks/common/instrument"
	"github.com/weaveworks/common/logging"
	"github.com/weaveworks/common/user"
	"github.com/weaveworks/service/billing-api/db"
	"github.com/weaveworks/service/billing-uploader/job/usage"
	timeutil "github.com/weaveworks/service/common/time"
	"github.com/weaveworks/service/users"
)

var (
	instancesCount = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "billing",
		Subsystem: "uploader",
		Name:      "billable_instances",
		Help:      "Number of billable instances in latest upload",
	}, []string{"uploader", "status"})
	recordsCount = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "billing",
		Subsystem: "uploader",
		Name:      "records",
		Help:      "Number of aggregated records in latest upload",
	}, []string{"uploader", "status", "amount_type"})
	amountsCount = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "billing",
		Subsystem: "uploader",
		Name:      "amounts",
		Help:      "Sum of aggregated values in latest upload",
	}, []string{"uploader", "status", "amount_type"})
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
	uploader  usage.Uploader
	collector *instrument.JobCollector
}

// NewUsageUpload instantiates UsageUpload.
func NewUsageUpload(db db.DB, users users.UsersClient, uploader usage.Uploader, collector *instrument.JobCollector) *UsageUpload {
	return &UsageUpload{
		db:        db,
		users:     users,
		uploader:  uploader,
		collector: collector,
	}
}

// Run starts the job and logs errors.
func (j *UsageUpload) Run() {
	if err := j.Do(); err != nil {
		log.Errorf("Error running upload job [%v]: %v", j.uploader.ID(), err)
	}
}

type uploadStats struct {
	count     map[string]int64 // Maps amount type to number of instances
	sum       map[string]int64 // Maps amount type to total value
	instances int              // Total number of instances

	// Highest aggregate ID seen is tracked to record up to which aggregate the
	// job has uploaded.
	maxAggregateID int
}

func (us *uploadStats) record(aggs []db.Aggregate) {
	if us.count == nil {
		us.count = make(map[string]int64)
	}
	if us.sum == nil {
		us.sum = make(map[string]int64)
	}
	for _, agg := range aggs {
		if agg.ID > us.maxAggregateID {
			us.maxAggregateID = agg.ID
		}
		us.count[agg.AmountType]++
		us.sum[agg.AmountType] += agg.AmountValue
	}

	us.instances++
}

func (us *uploadStats) set(uploader, status string) {
	instancesCount.WithLabelValues(uploader, status).Set(float64(us.instances))
	for amountType, records := range us.count {
		recordsCount.WithLabelValues(uploader, status, amountType).Set(float64(records))
	}
	for amountType, amountValue := range us.sum {
		amountsCount.WithLabelValues(uploader, status, amountType).Set(float64(amountValue))
	}
}

// Do starts the job and returns an error if it fails.
func (j *UsageUpload) Do() error {
	method := fmt.Sprintf("UsageUpload.Do(%s)", j.uploader.ID())
	return instrument.CollectedRequest(context.Background(), method, j.collector, nil, func(ctx context.Context) error {
		logger := logging.With(ctx)

		now := time.Now().UTC()
		through := j.uploader.ThroughTime(now)
		// We only process buckets that were fully aggregated. Buckets are aggregated continuously
		// by the hour. Aggregates with bucket_start equal "current time truncated to the hour" are still
		// in the process of receiving more aggregates. Since db.GetAggregatesAfter only selects aggregates
		// with bucket_start < through, we will only receive completed buckets.
		through = through.Truncate(1 * time.Hour)

		// Go back at most one week
		earliest := through.Add(-7 * 24 * time.Hour)

		// Reset previous report
		j.uploader.Reset()

		// Look up the billing-enabled instances where the trial has expired.
		resp, err := j.users.GetBillableOrganizations(ctx, &users.GetBillableOrganizationsRequest{
			Now: through,
		})
		if err != nil {
			logger.Errorf("Failed getting organizations: %v", err)
			return err
		}

		logger = logger.WithField("uploader", j.uploader.ID())

		startAggregateID, err := j.db.GetUsageUploadLargestAggregateID(ctx, j.uploader.ID())
		if err != nil {
			logger.Errorf("Failed reading aggregate ID: %v", err)
			return err
		}
		logger.Infof("Looking at usage between aggregate_id>%d and bucket_start<%v", startAggregateID, through)

		stats := uploadStats{}
		for _, org := range resp.Organizations {
			// Skip if uploader is not interested in this organization
			// TODO: move this filter to users.GetBillableOrganizations()
			if !j.uploader.IsSupported(org) {
				continue
			}

			orgCtx := user.InjectOrgID(ctx, org.ID)
			orgLogger := logging.With(orgCtx).WithField("uploader", j.uploader.ID())
			// Usage during trial is not uploaded
			orgFrom := timeutil.MaxTime(earliest, org.TrialExpiresAt)

			// Skip if their trial hasn't expired by the end of this period.
			// GetBillableOrganizations really shouldn't include any such
			// trials, but it's good to double-check.
			if org.InTrialPeriod(through) {
				orgLogger.Warn("Organization returned as 'billable' but trial still ongoing")
				continue
			}
			if org.ID == "" {
				orgLogger.Errorf("Internal instance ID is missing for %v", org.ExternalID)
				// We do not abort here because it's a persisting issue with a single account. That
				// shouldn't hold up the usage upload of all other accounts.
				continue
			}

			aggs, err := j.db.GetAggregatesAfter(ctx, org.ID, orgFrom, through, startAggregateID)
			if err != nil {
				return errors.Wrap(err, "error querying aggregates database")
			}

			orgLogger.Infof("Found %d aggregates for %v", len(aggs), org.ExternalID)
			if len(aggs) == 0 {
				continue
			}

			if err := j.uploader.Add(ctx, org, orgFrom, through, aggs); err != nil {
				return errors.Wrapf(err, "cannot add aggregates to %v", org.ExternalID)
			}

			stats.record(aggs)
		}

		logger.Infof("Found %d billable instances", stats.instances)

		if stats.instances > 0 {
			if err := j.upload(ctx, stats.maxAggregateID); err != nil {
				logger.Errorf("Failed uploading: %v", err)
				stats.set(j.uploader.ID(), "error")
				return err
			}
		}

		stats.set(j.uploader.ID(), "success")
		return nil
	})
}

// upload sends collected usage data. It also keeps track by recording in the database
// up to which aggregate ID it has uploaded.
func (j *UsageUpload) upload(ctx context.Context, maxAggregateID int) error {
	uploadID, err := j.db.InsertUsageUpload(ctx, j.uploader.ID(), maxAggregateID)
	if err != nil {
		return err
	}
	if err = j.uploader.Upload(ctx); err != nil {
		// Delete upload record because we failed, so our next run will picks these aggregates up again.
		if e := j.db.DeleteUsageUpload(ctx, j.uploader.ID(), uploadID); e != nil {
			// We couldn't delete the record of uploading usage and therefore will not retry in another run.
			// Manual intervention is required.
			return errors.Wrapf(e, "cannot delete usage upload entry (id=%v, max_id=%v, uploader=%v) after upload failed, you *must* delete this record manually before the next run", uploadID, maxAggregateID, j.uploader.ID())
		}
		return err
	}

	return nil
}
