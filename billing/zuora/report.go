package zuora

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/weaveworks/service/billing/db"
	timeutil "github.com/weaveworks/service/billing/util/time"
)

// Report is a usage report for Zuora.
type Report struct {
	entries []lineEntry
	Config
}

var reportHeader = []string{"ACCOUNT_ID", "UOM", "QTY", "STARTDATE", "ENDDATE", "SUBSCRIPTION_ID", "CHARGE_ID", "DESCRIPTION"}

type lineEntry struct {
	// Taken from: https://knowledgecenter.zuora.com/DC_Developers/REST_API/B_REST_API_reference/Usage/1_POST_usage
	accountID      string // Account number. Required.
	unitType       string // This must match the UOM for the usage that is set up in Zuora > username > Settings > Billing > Customize Units of Measure. Required
	quantity       string // Quantity. Required.
	startDate      string // This date determines the invoice item service period the associated usage is billed to. Date format is based on locale of the current user. Default: MM/DD/YYYY. Required
	endDate        string // See STARTDATE. Says optional, but Required? Eh?
	subscriptionID string // subscription ID to charge to. If empty, added to all user subscriptions. Required? Eh?
	chargeID       string // You can see the charge ID, e.g., C-00000001, when you add your rate plan to your subscription. Required.
	description    string // Optional description.
}

func (e lineEntry) toArray() []string {
	return []string{e.accountID, e.unitType, e.quantity, e.startDate, e.endDate, e.subscriptionID, e.chargeID, e.description}
}

// NewReport creates a new report.
func NewReport(cfg Config) *Report {
	if cfg.DateFormat == "" {
		cfg.DateFormat = "01/02/2006" // Default zuora config: MM/DD/YYYY
	}
	return &Report{
		entries: []lineEntry{},
		Config:  cfg,
	}
}

// AddLineEntry adds a line entry to the report.
func (r *Report) AddLineEntry(id, unitType string, units int64, startDate time.Time, endDate time.Time, subscriptionID, chargeID string) {
	truncatedStart := startDate.Format(r.DateFormat)
	if !midnight(startDate) {
		log.Warnf("Truncating usage start time to midnight: %v -> %v", startDate, truncatedStart)
	}
	truncatedEnd := endDate.Format(r.DateFormat)
	if !midnight(endDate) {
		log.Warnf("Truncating usage end time to midnight: %v -> %v", endDate, truncatedEnd)
	}
	r.entries = append(r.entries, r.newLineEntry(id, unitType, fmt.Sprintf("%v", units), truncatedStart, truncatedEnd, subscriptionID, chargeID))
}

// ToZuoraFormat converts the report to Zuora format.
func (r *Report) ToZuoraFormat() (io.Reader, error) {
	var result bytes.Buffer
	w := csv.NewWriter(&result)
	w.Write(reportHeader)
	for _, entry := range r.entries {
		if err := w.Write(entry.toArray()); err != nil {
			log.Errorf("error writing record to csv: %v", err)
			return nil, err
		}
	}
	w.Flush()
	return &result, w.Error()
}

func (r *Report) newLineEntry(id, unitType, quantity, startDate, endDate string, subscriptionID, chargeID string) lineEntry {
	return lineEntry{
		accountID:      id,
		unitType:       unitType,
		quantity:       quantity,
		startDate:      startDate,
		endDate:        endDate,
		subscriptionID: subscriptionID,
		chargeID:       chargeID,
		description:    description(unitType, quantity),
	}
}

func description(unitType, quantity string) string {
	if unitType != timeutil.NodeSeconds {
		// Not sure we want to attempt conversions for anything else than "node-seconds" at the moment.
		return generatedByBillingUploader()
	}
	quantityAsUint, err := strconv.ParseUint(quantity, 10, 64)
	if err != nil {
		log.Errorf("Failed to parse quantity [%v] to generate human-friendly description. Falling back on simpler description. Error: %v", quantity, err)
		return generatedByBillingUploader()
	}
	duration := timeutil.Seconds{Amount: quantityAsUint}.ToMostReadableUnit()
	if duration.Unit() == timeutil.NodeSeconds {
		// No point adding to the description if we convert from "node-seconds" to "node-seconds".
		return generatedByBillingUploader()
	}
	log.Infof("Converted [%v %v] into [%v] to generate human-friendly description", quantity, unitType, duration)
	return fmt.Sprintf("Approx. usage: %v. %v", duration, generatedByBillingUploader())
}

func generatedByBillingUploader() string {
	return fmt.Sprint("Generated by billing/uploader on ", time.Now().UTC().Format(time.RFC3339))
}

func midnight(time time.Time) bool {
	return time.Hour() == 0 && time.Minute() == 0 && time.Second() == 0
}

// ConcatEntries returns a new report with the configuration of this report
// and the entries from both reports.
func (r *Report) ConcatEntries(o *Report) *Report {
	return &Report{
		entries: append(r.entries, o.entries...),
		Config:  r.Config,
	}
}

// Size returns the number of entries.
func (r *Report) Size() int {
	return len(r.entries)
}

type groupKey struct {
	amountType string
	bucketTime time.Time
}

// ReportFromAggregates groups usage by 'billing period' (monthly in our case) for Zuora to generate correct invoices.
func ReportFromAggregates(config Config, aggs []db.Aggregate, paymentProviderID string, from, through time.Time, subscriptionNumber string, chargeNumber string, cycleDay int) (*Report, error) {
	r := NewReport(config)

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

	intervals, err := timeutil.MonthlyIntervalsWithinRange(from, through, cycleDay)
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