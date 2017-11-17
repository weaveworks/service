package usage

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"github.com/weaveworks/service/billing-api/db"
	"github.com/weaveworks/service/common/zuora"
	"github.com/weaveworks/service/users"
	"time"
)

// Zuora sends usage data to Zuora. It implements Uploader.
type Zuora struct {
	cl zuora.Client
	r  *zuora.Report
}

// NewZuora creates a Zuora instance.
func NewZuora(client zuora.Client) *Zuora {
	return &Zuora{
		cl: client,
		r:  zuora.NewReport(client.GetConfig()),
	}
}

// ID identifies this uploader.
func (z *Zuora) ID() string {
	return "zuora"
}

// Add collects usage by grouping aggregates in billing periods.
func (z *Zuora) Add(ctx context.Context, org users.Organization, from, through time.Time, aggs []db.Aggregate) error {
	account, err := z.cl.GetAccount(ctx, org.ExternalID)
	if err != nil {
		return errors.Wrapf(err, "cannot get Zuora account")
	}
	if account.PaymentProviderID == "" {
		return fmt.Errorf("account has no Zuora payment provider")
	}

	subscriptionNumber := account.Subscription.SubscriptionNumber
	chargeNumber := account.Subscription.ChargeNumber
	orgReport, err := zuora.ReportFromAggregates(z.cl.GetConfig(), aggs, account.PaymentProviderID, from, through, subscriptionNumber, chargeNumber, zuora.BillCycleDay)
	if err != nil {
		return errors.Wrap(err, "cannot create report")
	}
	z.r = z.r.ConcatEntries(orgReport)
	return nil
}

// Upload sends usage to Zuora.
func (z *Zuora) Upload(ctx context.Context) error {
	reader, err := z.r.ToZuoraFormat()
	if err != nil {
		return err
	}
	if _, err = z.cl.UploadUsage(ctx, reader); err != nil {
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
