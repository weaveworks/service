package users

import (
	"fmt"
	"testing"
	"time"
)

func asDays(days int) time.Duration {
	return time.Duration(days*24) * time.Hour
}

func TestTrialExpiry(t *testing.T) {
	now := time.Now().UTC()
	for _, example := range []struct {
		name         string
		createdAt    time.Time
		featureFlags []string
		end          time.Time
		err          error
	}{
		{
			name:         "basic",
			createdAt:    now.Add(asDays(-55) + 1*time.Hour),
			featureFlags: []string{"trial:days=60"},
			end:          now.Add(asDays(5) + 1*time.Hour),
			err:          nil,
		},
		{
			name:         "expired",
			createdAt:    now.Add(asDays(-61)),
			featureFlags: []string{"trial:days=60"},
			end:          now.Add(asDays(-1)),
			err:          nil,
		},
		{
			name:         "less than a day left",
			createdAt:    now.Add(asDays(-10) + 1*time.Hour),
			featureFlags: []string{"trial:days=10"},
			end:          now.Add(1 * time.Hour),
			err:          nil,
		},
		{
			name:         "no tag",
			createdAt:    now.Add(asDays(-10) + 1*time.Hour),
			featureFlags: []string{},
			end:          now.Add(asDays(20) + 1*time.Hour),
			err:          nil,
		},
		{
			name:         "malformed tag",
			createdAt:    now.Add((-10*24)*time.Hour + 1*time.Hour),
			featureFlags: []string{"trial:garbage=123"},
			end:          now.Add(asDays(20) + 1*time.Hour),
			err:          nil,
		},
		{
			name:         "non-numeric tag",
			createdAt:    time.Time{},
			featureFlags: []string{"trial:days=NAN!"},
			end:          time.Time{},
			err:          fmt.Errorf(`strconv.Atoi: parsing "NAN!": invalid syntax`),
		},
	} {
		gotTrialEnd, gotErr := CalculateTrialExpiry(example.createdAt, example.featureFlags)
		if fmt.Sprint(gotTrialEnd) != fmt.Sprint(example.end) {
			t.Errorf("[%s]\nExpected end: %#v\n     Got end: %#v", example.name, example.end, gotTrialEnd)
		}
		if fmt.Sprint(example.err) != fmt.Sprint(gotErr) {
			t.Errorf("[%s]\nExpected error: %v\n     Got error: %v", example.name, example.err, gotErr)
		}
	}
}
