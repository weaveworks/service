package time_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/weaveworks/service/billing/util/time"
)

func TestDurationGetAmount(t *testing.T) {
	assert.Equal(t, uint64(1), time.Seconds{Amount: 1}.GetAmount())
	assert.Equal(t, uint64(2), time.Minutes{Amount: 2}.GetAmount())
	assert.Equal(t, uint64(3), time.Hours{Amount: 3}.GetAmount())
	assert.Equal(t, uint64(4), time.Days{Amount: 4}.GetAmount())
}
func TestDurationUnit(t *testing.T) {
	assert.Equal(t, "node-seconds", time.Seconds{Amount: 1}.Unit())
	assert.Equal(t, time.NodeSeconds, time.Seconds{Amount: 1}.Unit())
	assert.Equal(t, "node-minutes", time.Minutes{Amount: 1}.Unit())
	assert.Equal(t, time.NodeMinutes, time.Minutes{Amount: 1}.Unit())
	assert.Equal(t, "node-hours", time.Hours{Amount: 1}.Unit())
	assert.Equal(t, time.NodeHours, time.Hours{Amount: 1}.Unit())
	assert.Equal(t, "node-days", time.Days{Amount: 1}.Unit())
	assert.Equal(t, time.NodeDays, time.Days{Amount: 1}.Unit())
}

func TestDurationToSeconds(t *testing.T) {
	assert.Equal(t, time.Seconds{Amount: 2}, time.Seconds{Amount: 2}.ToSeconds())
	assert.Equal(t, time.Seconds{Amount: 2 * 60}, time.Minutes{Amount: 2}.ToSeconds())
	assert.Equal(t, time.Seconds{Amount: 2 * 60 * 60}, time.Hours{Amount: 2}.ToSeconds())
	assert.Equal(t, time.Seconds{Amount: 2 * 60 * 60 * 24}, time.Days{Amount: 2}.ToSeconds())
}

func TestDurationToMinutes(t *testing.T) {
	assert.Equal(t, time.Minutes{Amount: 0}, time.Seconds{Amount: 2}.ToMinutes())
	assert.Equal(t, time.Minutes{Amount: 2}, time.Seconds{Amount: 2 * 60}.ToMinutes())
	assert.Equal(t, time.Minutes{Amount: 2}, time.Minutes{Amount: 2}.ToMinutes())
	assert.Equal(t, time.Minutes{Amount: 2 * 60}, time.Hours{Amount: 2}.ToMinutes())
	assert.Equal(t, time.Minutes{Amount: 2 * 60 * 24}, time.Days{Amount: 2}.ToMinutes())
}

func TestDurationToHours(t *testing.T) {
	assert.Equal(t, time.Hours{Amount: 0}, time.Seconds{Amount: 2}.ToHours())
	assert.Equal(t, time.Hours{Amount: 2}, time.Seconds{Amount: 2 * 60 * 60}.ToHours())
	assert.Equal(t, time.Hours{Amount: 0}, time.Minutes{Amount: 2}.ToHours())
	assert.Equal(t, time.Hours{Amount: 2}, time.Minutes{Amount: 2 * 60}.ToHours())
	assert.Equal(t, time.Hours{Amount: 2}, time.Hours{Amount: 2}.ToHours())
	assert.Equal(t, time.Hours{Amount: 2 * 24}, time.Days{Amount: 2}.ToHours())
}

func TestDurationToDays(t *testing.T) {
	assert.Equal(t, time.Days{Amount: 0}, time.Seconds{Amount: 2}.ToDays())
	assert.Equal(t, time.Days{Amount: 2}, time.Seconds{Amount: 2 * 60 * 60 * 24}.ToDays())
	assert.Equal(t, time.Days{Amount: 0}, time.Minutes{Amount: 2}.ToDays())
	assert.Equal(t, time.Days{Amount: 2}, time.Minutes{Amount: 2 * 60 * 24}.ToDays())
	assert.Equal(t, time.Days{Amount: 0}, time.Hours{Amount: 2}.ToDays())
	assert.Equal(t, time.Days{Amount: 2}, time.Hours{Amount: 2 * 24}.ToDays())
	assert.Equal(t, time.Days{Amount: 2}, time.Days{Amount: 2}.ToDays())
}

func TestSecondsToMostReadableUnit(t *testing.T) {
	assert.Equal(t, time.Seconds{Amount: 0}, time.Seconds{Amount: 0}.ToMostReadableUnit())
	assert.Equal(t, time.Seconds{Amount: 1}, time.Seconds{Amount: 1}.ToMostReadableUnit())
	assert.Equal(t, time.Seconds{Amount: 59}, time.Seconds{Amount: 59}.ToMostReadableUnit())
	assert.Equal(t, time.Minutes{Amount: 1}, time.Seconds{Amount: 60}.ToMostReadableUnit())
	assert.Equal(t, time.Minutes{Amount: 59}, time.Seconds{Amount: 3599}.ToMostReadableUnit())
	assert.Equal(t, time.Hours{Amount: 1}, time.Seconds{Amount: 3600}.ToMostReadableUnit())
	assert.Equal(t, time.Hours{Amount: 23}, time.Seconds{Amount: 86399}.ToMostReadableUnit())
	assert.Equal(t, time.Days{Amount: 1}, time.Seconds{Amount: 86400}.ToMostReadableUnit())
	assert.Equal(t, time.Days{Amount: 40}, time.Seconds{Amount: 3456000}.ToMostReadableUnit())

	assert.Equal(t, time.Minutes{Amount: 0}, time.Minutes{Amount: 0}.ToMostReadableUnit())
	assert.Equal(t, time.Minutes{Amount: 1}, time.Minutes{Amount: 1}.ToMostReadableUnit())
	assert.Equal(t, time.Minutes{Amount: 59}, time.Minutes{Amount: 59}.ToMostReadableUnit())
	assert.Equal(t, time.Hours{Amount: 1}, time.Minutes{Amount: 60}.ToMostReadableUnit())
	assert.Equal(t, time.Hours{Amount: 23}, time.Minutes{Amount: 1439}.ToMostReadableUnit())
	assert.Equal(t, time.Days{Amount: 1}, time.Minutes{Amount: 1440}.ToMostReadableUnit())
	assert.Equal(t, time.Days{Amount: 40}, time.Minutes{Amount: 57600}.ToMostReadableUnit())

	assert.Equal(t, time.Hours{Amount: 0}, time.Hours{Amount: 0}.ToMostReadableUnit())
	assert.Equal(t, time.Hours{Amount: 1}, time.Hours{Amount: 1}.ToMostReadableUnit())
	assert.Equal(t, time.Hours{Amount: 23}, time.Hours{Amount: 23}.ToMostReadableUnit())
	assert.Equal(t, time.Days{Amount: 1}, time.Hours{Amount: 24}.ToMostReadableUnit())
	assert.Equal(t, time.Days{Amount: 40}, time.Hours{Amount: 960}.ToMostReadableUnit())

	assert.Equal(t, time.Days{Amount: 0}, time.Days{Amount: 0}.ToMostReadableUnit())
	assert.Equal(t, time.Days{Amount: 1}, time.Days{Amount: 1}.ToMostReadableUnit())
	assert.Equal(t, time.Days{Amount: 40}, time.Days{Amount: 40}.ToMostReadableUnit())
}

func TestDurationToString(t *testing.T) {
	assert.Equal(t, "2 node-seconds", time.Seconds{Amount: 2}.String())
	assert.Equal(t, "2 node-minutes", time.Minutes{Amount: 2}.String())
	assert.Equal(t, "2 node-hours", time.Hours{Amount: 2}.String())
	assert.Equal(t, "2 node-days", time.Days{Amount: 2}.String())
}
