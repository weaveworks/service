package usage

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"

	"github.com/weaveworks/service/billing-api/db"
	"github.com/weaveworks/service/common/zuora"
	"github.com/weaveworks/service/users"
)

// Zuora sends usage data to Zuora. It implements Uploader.
type Zuora struct {
	cl zuora.Client
	r  *zuora.Report
}

// NewZuora creates a Zuora instance.
func NewZuora(client zuora.Client) *Zuora {
	z := &Zuora{
		cl: client,
	}
	z.Reset()
	return z
}

// ID identifies this uploader.
func (z *Zuora) ID() string {
	return "zuora"
}

// Reset replaces the current report with an empty one
func (z *Zuora) Reset() {
	z.r = zuora.NewReport(z.cl.GetConfig())
}

// Add collects usage by grouping aggregates in billing periods.
func (z *Zuora) Add(ctx context.Context, org users.Organization, from, through time.Time, aggs []db.Aggregate) error {
	account, err := z.cl.GetAccount(ctx, org.ZuoraAccountNumber)
	if err != nil {
		return errors.Wrapf(err, "cannot get Zuora account")
	}
	if account.PaymentProviderID == "" {
		return fmt.Errorf("account has no Zuora payment provider")
	}

	subscriptionNumber := account.Subscription.SubscriptionNumber
	chargeNumber := account.Subscription.ChargeNumber

	aggs, err = FilterAggregatesForSubscription(ctx, z.cl, aggs, account)
	if err != nil {
		return err
	}

	orgReport, err := zuora.ReportFromAggregates(z.cl.GetConfig(), aggs, account.PaymentProviderID, minBucketStart(aggs), through, subscriptionNumber, chargeNumber, zuora.BillCycleDay)
	if err != nil {
		return errors.Wrap(err, "cannot create report")
	}
	z.r = z.r.ConcatEntries(orgReport)
	return nil
}

func minBucketStart(aggs []db.Aggregate) time.Time {
	var l time.Time
	for _, a := range aggs {
		if l.IsZero() || a.BucketStart.Before(l) {
			l = a.BucketStart
		}
	}
	return l
}

// Upload sends usage to Zuora.
func (z *Zuora) Upload(ctx context.Context, id string) error {
	reader, err := z.r.ToZuoraFormat()
	if err != nil {
		return err
	}
	if _, err = z.cl.UploadUsage(ctx, reader, id); err != nil {
		return err
	}
	return nil
}

// IsSupported returns true for organizations that have a Zuora account number.
func (z *Zuora) IsSupported(org users.Organization) bool {
	return org.ZuoraAccountNumber != ""
}

// ThroughTime returns time of the previous midnight.
func (z *Zuora) ThroughTime(now time.Time) time.Time {
	return now.Truncate(24 * time.Hour)
}
