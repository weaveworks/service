package routes

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
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
}
