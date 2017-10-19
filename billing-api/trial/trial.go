package trial

import (
	"math"
	"time"

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
	length := o.TrialExpiresAt.Sub(o.CreatedAt)

	// Remaining is the expiry - time now
	remainingTime := o.TrialExpiresAt.Sub(now)
	remainingDays := days(remainingTime)

	// Ceil because if expires in a few hours, we still have 1 day left.
	// Max because if time is negative, then just report 0 days left
	remaining := int(math.Max(0, math.Ceil(remainingDays)))

	return Trial{
		Length:    int(days(length)),
		Remaining: remaining,
		Start:     o.CreatedAt,
		End:       o.TrialExpiresAt,
	}
}

func days(duration time.Duration) float64 {
	return duration.Hours() / 24.0
}
