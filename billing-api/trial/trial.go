package trial

import (
	"math"
	"time"

	common_time "github.com/weaveworks/service/common/time"
	"github.com/weaveworks/service/users"
)

const (
	trialFlag          string = "trial:days"
	defaultTrialLength int    = 30
)

// Trial is a bundle of information about the trial period used by the frontend.
type Trial struct {
	// Length is the original length of trial period. This isn't actually
	// used. TODO(jml): Remove this field once weaveworks/service-ui#1037 is
	// deployed to production.
	Length int `json:"length"`
	// Remaining is the number of days remaining, rounded to whole days.
	Remaining int `json:"remaining"`
	// Start is when the trial started.
	Start time.Time `json:"start"`
	// End is when the trial ended / will end.
	End time.Time `json:"end"`
}

// Info returns a bundle of information about the trial period that gets
// used in the Javascript frontend.
func Info(o users.Organization, now time.Time) Trial {
	return Trial{
		Length:    Length(o.TrialExpiresAt, o.CreatedAt),
		Remaining: Remaining(o.TrialExpiresAt, now),
		// An instance may be created after the trial expires. In that case,
		// we just return the same time for Start and End. This should be
		// caught in the frontend by length == 0.
		Start: common_time.MinTime(o.CreatedAt, o.TrialExpiresAt),
		End:   o.TrialExpiresAt,
	}
}

// Remaining computes the remaining days available in the trial.
func Remaining(expires, now time.Time) int {
	return days(expires.Sub(now))
}

// Length computes the trial length in days.
func Length(expires, created time.Time) int {
	return days(expires.Sub(created))
}

// Days returns number of days in a duration. It rounds up to the next day and
// will return 0 for negative durations.
func days(duration time.Duration) int {
	return int(math.Max(math.Ceil(duration.Hours()/24.0), 0))
}
