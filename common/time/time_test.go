package time

import (
	"testing"
	"time"
)

const (
	layout string = "2006-01-02 15:04:05"
)

func ts(t *testing.T, str string) time.Time {
	timestamp, err := time.Parse(layout, str)
	if err != nil {
		t.Fatal("Failed to parse timestamp", err)
	}
	return timestamp
}

func TestDaysIn(t *testing.T) {
	for result, expected := range map[int]int{
		DaysIn(1, 2017):  31,
		DaysIn(2, 2017):  28,
		DaysIn(3, 2017):  31,
		DaysIn(4, 2017):  30,
		DaysIn(5, 2017):  31,
		DaysIn(6, 2017):  30,
		DaysIn(7, 2017):  31,
		DaysIn(8, 2017):  31,
		DaysIn(9, 2017):  30,
		DaysIn(10, 2017): 31,
		DaysIn(11, 2017): 30,
		DaysIn(12, 2017): 31,
		DaysIn(2, 2000):  29,
		DaysIn(2, 2008):  29,
		DaysIn(2, 2115):  28,
		DaysIn(2, 2196):  29,
	} {
		if result != expected {
			t.Fatalf("Result not expected: %v != %v", result, expected)
		}
	}
}

func TestBeginningOfMonth(t *testing.T) {
	for input, expectedStr := range map[string]string{
		"2017-09-13 12:30:32": "2017-09-01 00:00:00",
		"2017-09-13 00:00:01": "2017-09-01 00:00:00",
		"2016-02-29 23:59:59": "2016-02-01 00:00:00",
		"2016-07-28 13:54:43": "2016-07-01 00:00:00",
	} {
		timestamp := ts(t, input)
		expected := ts(t, expectedStr)
		result := BeginningOfMonth(timestamp)
		if !result.Equal(expected) {
			t.Fatalf("Result not expected: %v != %v", result, expected)
		}
	}
}

func TestBeginningOfNextMonth(t *testing.T) {
	for input, expectedStr := range map[string]string{
		"2017-09-13 12:30:32": "2017-10-01 00:00:00",
		"2017-09-13 00:00:01": "2017-10-01 00:00:00",
		"2016-02-29 23:59:59": "2016-03-01 00:00:00",
		"2016-07-28 13:54:43": "2016-08-01 00:00:00",
		"2017-12-28 13:54:43": "2018-01-01 00:00:00",
	} {
		timestamp := ts(t, input)
		expected := ts(t, expectedStr)
		result := BeginningOfNextMonth(timestamp)
		if !result.Equal(expected) {
			t.Fatalf("Result not expected: %v != %v", result, expected)
		}
	}
}

func TestEndOfMonth(t *testing.T) {
	for input, expectedStr := range map[string]string{
		"2017-09-13 12:30:32": "2017-09-30 23:59:59",
		"2017-09-13 00:00:01": "2017-09-30 23:59:59",
		"2016-02-24 23:59:59": "2016-02-29 23:59:59",
		"2017-02-24 23:59:59": "2017-02-28 23:59:59",
		"2016-07-28 13:54:43": "2016-07-31 23:59:59",
		"2017-12-28 13:54:43": "2017-12-31 23:59:59",
	} {
		timestamp := ts(t, input)
		expected := ts(t, expectedStr)
		result := EndOfMonth(timestamp)
		// compare nanosecond first (it's difficult to parse them)
		if result.Nanosecond() != 999999999 {
			t.Fatalf("Nanoseconds incorrect: %v", result)
		}
		result = result.Truncate(time.Second)
		if !result.Equal(expected) {
			t.Fatalf("Result not expected: %v != %v", result, expected)
		}
	}
}

func TestMinTime(t *testing.T) {
	for minStr, maxStr := range map[string]string{
		"2017-09-13 12:30:32": "2020-01-01 00:00:00",
		"2017-09-13 00:00:01": "2017-09-13 00:00:01",
		"2016-02-29 23:59:59": "2016-03-01 00:00:00",
		"2016-07-28 13:54:43": "2017-03-14 16:23:40",
		"2017-12-28 13:54:43": "2018-01-01 00:00:00",
	} {
		min := ts(t, minStr)
		max := ts(t, maxStr)
		if !MinTime(min, max).Equal(min) {
			t.Fatalf("Result not expected: %v != %v", max, min)
		}
	}
}

func TestInTimeRange(t *testing.T) {
	for inputStr, interval := range map[string]struct {
		from string
		to   string
	}{
		"2017-09-13 12:30:32": {from: "2017-09-10 20:23:44", to: "2020-01-01 00:00:00"},
		"2017-09-13 00:00:01": {from: "2000-09-10 20:23:44", to: "2117-09-13 00:00:01"},
		"2016-02-29 23:59:59": {from: "2016-02-29 23:59:59", to: "2016-03-01 00:00:00"},
		"2016-07-28 13:54:43": {from: "2016-06-10 20:23:44", to: "2016-08-14 16:23:40"},
	} {
		input := ts(t, inputStr)
		from := ts(t, interval.from)
		to := ts(t, interval.to)
		if !InTimeRange(from, to, input) {
			t.Fatalf("Result not expected, the following is not true: %v <= %v < %v", from, input, to)
		}
	}
}

func TestMonthlyIntervalsWithinRange(t *testing.T) {
	// first case: no overlapping month
	from := ts(t, "2017-09-13 12:30:32")
	to := ts(t, "2017-09-24 23:43:12")
	intervals, err := MonthlyIntervalsWithinRange(from, to, 1)
	if err != nil {
		t.Fatal("Failed to run MonthlyIntervalsWithinRange", err)
	}
	if len(intervals) != 1 {
		t.Fatalf("Expected intervals len 1, got: %v", len(intervals))
	}
	interval := intervals[0]
	if !from.Equal(interval.From) {
		t.Fatalf("Result not expected: %v != %v", from, interval.From)
	}
	if !to.Equal(interval.To) {
		t.Fatalf("Result not expected: %v != %v", to, interval.To)
	}

	// second case: cross the year
	from = ts(t, "2017-11-13 12:30:32")
	to = ts(t, "2018-02-24 23:43:12")
	intervals, err = MonthlyIntervalsWithinRange(from, to, 1)
	if err != nil {
		t.Fatal("Failed to run MonthlyIntervalsWithinRange", err)
	}
	if len(intervals) != 4 {
		t.Fatalf("Expected intervals len 4, got: %v", len(intervals))
	}
	interval = intervals[0]
	if !from.Equal(interval.From) {
		t.Fatalf("Result not expected: %v != %v", from, interval.From)
	}
	expectedTo := ts(t, "2017-12-01 00:00:00")
	if !expectedTo.Equal(interval.To) {
		t.Fatalf("Result not expected: %v != %v", expectedTo, interval.To)
	}
	interval = intervals[1]
	expectedFrom := expectedTo
	if !expectedFrom.Equal(interval.From) {
		t.Fatalf("Result not expected: %v != %v", expectedFrom, interval.From)
	}
	expectedTo = ts(t, "2018-01-01 00:00:00")
	if !expectedTo.Equal(interval.To) {
		t.Fatalf("Result not expected: %v != %v", expectedTo, interval.To)
	}
	interval = intervals[2]
	expectedFrom = expectedTo
	if !expectedFrom.Equal(interval.From) {
		t.Fatalf("Result not expected: %v != %v", expectedFrom, interval.From)
	}
	expectedTo = ts(t, "2018-02-01 00:00:00")
	if !expectedTo.Equal(interval.To) {
		t.Fatalf("Result not expected: %v != %v", expectedTo, interval.To)
	}
	interval = intervals[3]
	expectedFrom = expectedTo
	if !expectedFrom.Equal(interval.From) {
		t.Fatalf("Result not expected: %v != %v", expectedFrom, interval.From)
	}
	expectedTo = to
	if !expectedTo.Equal(interval.To) {
		t.Fatalf("Result not expected: %v != %v", expectedTo, interval.To)
	}

	// third case: align with start of month
	from = ts(t, "2017-11-01 00:00:00")
	to = ts(t, "2017-12-03 23:43:12")
	intervals, err = MonthlyIntervalsWithinRange(from, to, 1)
	if err != nil {
		t.Fatal("Failed to run MonthlyIntervalsWithinRange", err)
	}
	if len(intervals) != 2 {
		t.Fatalf("Expected intervals len 2, got: %v", len(intervals))
	}
	interval = intervals[0]
	expectedFrom = from
	if !expectedFrom.Equal(interval.From) {
		t.Fatalf("Result not expected: %v != %v", expectedFrom, interval.From)
	}
	expectedTo = ts(t, "2017-12-01 00:00:00")
	if !expectedTo.Equal(interval.To) {
		t.Fatalf("Result not expected: %v != %v", expectedTo, interval.To)
	}
	interval = intervals[1]
	expectedFrom = expectedTo
	if !expectedFrom.Equal(interval.From) {
		t.Fatalf("Result not expected: %v != %v", expectedFrom, interval.From)
	}
	expectedTo = to
	if !expectedTo.Equal(interval.To) {
		t.Fatalf("Result not expected: %v != %v", expectedTo, interval.To)
	}

	// fourth case: align with start of next month
	from = ts(t, "2017-11-23 13:32:12")
	to = ts(t, "2017-12-01 00:00:00")
	intervals, err = MonthlyIntervalsWithinRange(from, to, 1)
	if err != nil {
		t.Fatal("Failed to run MonthlyIntervalsWithinRange", err)
	}
	if len(intervals) != 1 {
		t.Fatalf("Expected intervals len 1, got: %v", len(intervals))
	}
	interval = intervals[0]
	expectedFrom = from
	if !expectedFrom.Equal(interval.From) {
		t.Fatalf("Result not expected: %v != %v", expectedFrom, interval.From)
	}
	expectedTo = to
	if !expectedTo.Equal(interval.To) {
		t.Fatalf("Result not expected: %v != %v", expectedTo, interval.To)
	}
}

func TestNextCycleDay(t *testing.T) {
	for input, expectedStr := range map[struct {
		dateStr  string
		cycleDay int
	}]string{
		{dateStr: "2017-09-13 12:30:32", cycleDay: 31}: "2017-09-30 00:00:00",
		{dateStr: "2017-09-13 12:30:32", cycleDay: 13}: "2017-10-13 00:00:00",
		{dateStr: "2017-09-13 12:30:32", cycleDay: 1}:  "2017-10-01 00:00:00",
		{dateStr: "2017-09-13 12:30:32", cycleDay: 10}: "2017-10-10 00:00:00",
		{dateStr: "2016-02-13 17:14:00", cycleDay: 31}: "2016-02-29 00:00:00",
		{dateStr: "2016-02-13 13:00:00", cycleDay: 30}: "2016-02-29 00:00:00",
		{dateStr: "2016-02-13 20:12:00", cycleDay: 29}: "2016-02-29 00:00:00",
		{dateStr: "2016-02-13 23:59:59", cycleDay: 28}: "2016-02-28 00:00:00",
		{dateStr: "2017-12-13 11:45:12", cycleDay: 31}: "2017-12-31 00:00:00",
		{dateStr: "2017-12-13 23:00:00", cycleDay: 6}:  "2018-01-06 00:00:00",
		{dateStr: "2017-11-30 00:00:00", cycleDay: 31}:  "2017-12-31 00:00:00",
	} {
		reference := ts(t, input.dateStr)
		expected := ts(t, expectedStr)
		result := NextCycleStart(reference, input.cycleDay)
		if !result.Equal(expected) {
			t.Fatalf("Result not expected: %v != %v", result, expected)
		}
	}
}
