package util

import (
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/weaveworks/service/billing/db"
	timeutil "github.com/weaveworks/service/billing/util/time"
	"github.com/weaveworks/service/billing/zuora"
)

type groupKey struct {
	amountType string
	bucketTime time.Time
}

// ReportFromAggregates groups usage by 'billing period' (monthly in our case) for Zuora to generate correct invoices.
func ReportFromAggregates(config zuora.Config, aggs []db.Aggregate, paymentProviderID string, from, through time.Time, subscriptionNumber string, chargeNumber string, cycleDay int) (*zuora.Report, error) {
	r := zuora.NewReport(config)

	// Sum them by (type,month).
	// This is because zuora requires usage to be grouped by month to issue invoices with charges corresponding to the correct period.
	// `bucketTime` is used for grouping and is therefore part a common key to all report lines which belong in the same month.
	groupedSums := map[groupKey]int64{}
	for _, agg := range aggs {
		// `bucketTime` is  bounded by `through`, but must be lower than `through` because intervals are [inclusive, exclusive)
		bucketTime := timeutil.MinTime(timeutil.EndOfCycle(agg.BucketStart, cycleDay), timeutil.JustBefore(through))
		key := groupKey{amountType: agg.AmountType, bucketTime: bucketTime}
		groupedSums[key] += agg.AmountValue
	}

	intervals, err := timeutil.MonthlyIntervalsWithinRangeByCycleDay(from, through, cycleDay)
	if err != nil {
		return nil, err
	}

	// Add this instance's sums to the report.
	// O(n^2)
	for key, amountValue := range groupedSums {
		added := false
		for _, interval := range intervals {
			if timeutil.InTimeRange(interval.From, interval.To, key.bucketTime) {
				r.AddLineEntry(paymentProviderID, key.amountType, amountValue, interval.From, interval.To, subscriptionNumber, chargeNumber)
				added = true
				break
			}
		}

		// Be pendantic: this should never log!
		if added == false {
			log.Errorf("Report line entry not added: %+v %v", key, amountValue)
		}
	}

	return r, nil
}
