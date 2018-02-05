package routes

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weaveworks/service/billing-api/db"
)

func date(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func TestGetBillingPeriod(t *testing.T) {
	epoch := time.Unix(0, 0)
	// basic in-period state
	start, end := computeBillingPeriod(5, epoch, epoch, date(2017, time.June, 6))
	require.Equal(t, date(2017, time.June, 5), start)
	require.Equal(t, date(2017, time.July, 5), end)

	// period first day
	start, end = computeBillingPeriod(5, epoch, epoch, date(2017, time.June, 5))
	require.Equal(t, date(2017, time.June, 5), start)
	require.Equal(t, date(2017, time.July, 5), end)

	// period last day
	start, end = computeBillingPeriod(5, epoch, epoch, date(2017, time.June, 4))
	require.Equal(t, date(2017, time.May, 5), start)
	require.Equal(t, date(2017, time.June, 5), end)

	// bill cycle on 31st, in month with 30 days
	start, end = computeBillingPeriod(31, epoch, epoch, date(2017, time.September, 1))
	require.Equal(t, date(2017, time.August, 31), start)
	require.Equal(t, date(2017, time.September, 30), end)

	// bill cycle on 31st, in a leap-year feb
	start, end = computeBillingPeriod(31, epoch, epoch, date(2016, time.March, 1))
	require.Equal(t, date(2016, time.February, 29), start)
	require.Equal(t, date(2016, time.March, 31), end)

	// bill cycle over year boundary
	start, end = computeBillingPeriod(24, epoch, epoch, date(2017, time.January, 1))
	require.Equal(t, date(2016, time.December, 24), start)
	require.Equal(t, date(2017, time.January, 24), end)

	// mid-trial
	start, end = computeBillingPeriod(
		1,
		date(2017, time.April, 10),
		date(2017, time.June, 10),
		date(2017, time.May, 1))
	require.Equal(t, date(2017, time.May, 1), start)
	require.Equal(t, date(2017, time.June, 1), end)

	// at the start of a trial
	start, end = computeBillingPeriod(
		1,
		date(2017, time.April, 10),
		date(2017, time.June, 10),
		date(2017, time.April, 20))
	require.Equal(t, date(2017, time.April, 10), start)
	require.Equal(t, date(2017, time.May, 1), end)

	// at the end of a trial
	start, end = computeBillingPeriod(
		1,
		date(2017, time.April, 10),
		date(2017, time.June, 10),
		date(2017, time.June, 2))
	require.Equal(t, date(2017, time.June, 1), start)
	require.Equal(t, date(2017, time.June, 10), end)

	// the day after a trial has ended
	start, end = computeBillingPeriod(
		1,
		date(2017, time.April, 5),
		date(2017, time.June, 1),
		date(2017, time.June, 1))
	require.Equal(t, date(2017, time.June, 1), start)
	require.Equal(t, date(2017, time.July, 1), end)

	// just after the trial has expired
	start, end = computeBillingPeriod(
		1,
		date(2017, time.May, 5),
		date(2017, time.June, 4),
		date(2017, time.June, 30))
	require.Equal(t, date(2017, time.June, 4), start)
	require.Equal(t, date(2017, time.July, 1), end)

	// trial end before created timestamp,
	// billing day before reference day
	start, end = computeBillingPeriod(
		1,
		date(2017, time.May, 10),
		date(2017, time.May, 3),
		date(2017, time.May, 25))
	require.Equal(t, date(2017, time.May, 10), start)
	require.Equal(t, date(2017, time.June, 1), end)

	// trial end before created timestamp.
	// billing day after reference day
	start, end = computeBillingPeriod(
		5,
		date(2017, time.May, 10),
		date(2017, time.May, 7),
		date(2017, time.May, 4))
	require.Equal(t, date(2017, time.May, 10), start)
	require.Equal(t, date(2017, time.May, 5), end)

	// trial end before created timestamp.
	// created at and trial end in previous periods.
	start, end = computeBillingPeriod(
		5,
		date(2017, time.March, 10),
		date(2017, time.March, 5),
		date(2017, time.May, 4))
	require.Equal(t, date(2017, time.April, 5), start)
	require.Equal(t, date(2017, time.May, 5), end)
}

func TestComputeEstimationPeriod(t *testing.T) {
	now := date(2017, time.December, 21)
	ago := now.Add(-7 * 24 * time.Hour)

	var from, to time.Time
	var days int

	{ // trial
		from, to, days = computeEstimationPeriod(now, now.Add(time.Hour))
		assert.Equal(t, ago, from)
		assert.Equal(t, now, to)
		assert.Equal(t, 7, days)
	}
	{ // first day after trial
		from, to, days = computeEstimationPeriod(now, now.Add(-time.Hour))
		assert.Equal(t, now, from)
		assert.Equal(t, now, to)
		assert.Equal(t, 0, days)
	}
	{ // with trial expiration within those 7d
		expires := time.Date(2017, 12, 18, 15, 0, 0, 0, time.UTC)
		expectedFrom := date(2017, time.December, 19)
		from, to, days = computeEstimationPeriod(now, expires)
		assert.Equal(t, expectedFrom, from) // end of expires day
		assert.Equal(t, now, to)
		assert.Equal(t, 2, days)
	}
	{ // defaults to 7d
		from, to, days = computeEstimationPeriod(now, time.Time{})
		assert.Equal(t, ago, from)
		assert.Equal(t, now, to)
		assert.Equal(t, 7, days)
	}
}

func TestEstimatedMonthlyUsage(t *testing.T) {
	ti := date(2017, time.December, 21)
	start := date(2017, time.December, 1)
	now := time.Date(2017, 12, 21, 12, 0, 0, 0, time.UTC)
	var usage float64
	daily := map[string]int64{
		"2017-12-01": 400,
		"2017-12-20": 600,
		"2017-12-21": 2000, // should be ignored
	}
	aggs := []db.Aggregate{
		{
			AmountType:  "node-seconds",
			AmountValue: 10,
			BucketStart: ti,
		},
		{
			AmountType:  "node-seconds",
			AmountValue: 20,
			BucketStart: ti.Add(time.Hour),
		},
	}
	{ // same day
		aggs[0].BucketStart = ti
		aggs[1].BucketStart = ti.Add(time.Hour)
		usage = estimatedMonthlyUsage(daily, start, aggs, 1, 1, now)
		assert.Equal(t, float64(1330), usage)

		// two days, second day had no usage
		usage = estimatedMonthlyUsage(daily, start, aggs, 2, 1, now)
		assert.Equal(t, float64(1165), usage)
	}
	{ // consecutive day
		aggs[0].BucketStart = ti
		aggs[1].BucketStart = ti.Add(24 * time.Hour)
		usage = estimatedMonthlyUsage(daily, start, aggs, 2, 1, now)
		assert.Equal(t, float64(1165), usage)

		// third day empty
		usage = estimatedMonthlyUsage(daily, start, aggs, 3, 1, now)
		assert.Equal(t, float64(1110), usage)
	}
	{ // skip day
		aggs[0].BucketStart = ti
		aggs[1].BucketStart = ti.Add(2 * 24 * time.Hour)
		usage = estimatedMonthlyUsage(daily, start, aggs, 3, 1, now)
		assert.Equal(t, float64(1110), usage)

		// fourth day empty
		usage = estimatedMonthlyUsage(daily, start, aggs, 4, 1, now)
		assert.Equal(t, float64(1082.5), usage)
	}
}
