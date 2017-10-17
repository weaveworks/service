package time

import (
	"errors"
	"math"
	"time"
)

// Interval represents a time range.
type Interval struct {
	From time.Time
	To   time.Time
}

// DaysIn returns how many days in the given month.
func DaysIn(m time.Month, year int) int {
	return time.Date(year, m+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// BeginningOfNextDay returns 00:00 of the next day.
func BeginningOfNextDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day()+1, 0, 0, 0, 0, t.Location())
}

// BeginningOfMonth returns 1st of month at 00:00.
func BeginningOfMonth(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
}

// BeginningOfNextMonth returns 00:00 of the 1st of next month.
func BeginningOfNextMonth(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, t.Location())
}

// EndOfMonth returns 1ns before BeginningOfNextMonth.
func EndOfMonth(t time.Time) time.Time {
	return JustBefore(BeginningOfNextMonth(t))
}

// JustBefore subtracts 1ns from the given time.
func JustBefore(t time.Time) time.Time {
	return t.Add(-time.Nanosecond)
}

// MinTime returns the one that is earlier.
func MinTime(lhs, rhs time.Time) time.Time {
	if lhs.Before(rhs) {
		return lhs
	}
	return rhs
}

// MaxTime returns the posterior time.
func MaxTime(lhs, rhs time.Time) time.Time {
	if lhs.Before(rhs) {
		return rhs
	}
	return lhs
}

// InTimeRange checks whether `t` is within [begin, end)
func InTimeRange(begin, end, t time.Time) bool {
	return !begin.After(t) && t.Before(end)
}

// NextCycleStart calculates when the next period starts regarding given cycleDay.
func NextCycleStart(reference time.Time, cycleDay int) time.Time {
	if cycleDay > reference.Day() {
		daysInMonth := DaysIn(reference.Month(), reference.Year())
		day := int(math.Min(float64(daysInMonth), float64(cycleDay)))
		return time.Date(reference.Year(), reference.Month(), day, 0, 0, 0, 0, reference.Location())
	}
	return time.Date(reference.Year(), reference.Month()+1, cycleDay, 0, 0, 0, 0, reference.Location())
}

// EndOfCycle returns 1ns before the next cycle starts.
func EndOfCycle(t time.Time, cycleDay int) time.Time {
	return JustBefore(NextCycleStart(t, cycleDay))
}

// MonthlyIntervalsWithinRange returns a slice of intervals bounded by `from` and `to`, where the start of an interval start on `day`
func MonthlyIntervalsWithinRange(from, to time.Time, cycleDay int) ([]Interval, error) {
	if !from.Before(to) {
		return nil, errors.New("from must be lower than to")
	}
	if cycleDay < 1 && cycleDay > 31 {
		return nil, errors.New("day must be within [1, 31]")
	}
	var intervals []Interval
	for begin := from; begin.Before(to); {
		end := MinTime(NextCycleStart(begin, cycleDay), to)
		intervals = append(intervals, Interval{From: begin, To: end})
		begin = end
	}
	return intervals, nil
}
