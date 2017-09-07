package users

import (
	"fmt"
	"testing"
	"time"
)

func asDays(days int) time.Duration {
	return time.Duration(days*24) * time.Hour
}

func TestTrialInfo(t *testing.T) {
	now := time.Now().UTC()
	for _, example := range []struct {
		name         string
		organization *Organization
		end          time.Time
		err          error
	}{
		{
			name: "basic",
			organization: &Organization{
				CreatedAt:    now.Add(asDays(-55) + 1*time.Hour),
				FeatureFlags: []string{"trial:days=60"},
			},
			end: now.Add(asDays(5) + 1*time.Hour),
			err: nil,
		},
		{
			name: "expired",
			organization: &Organization{
				CreatedAt:    now.Add(asDays(-61)),
				FeatureFlags: []string{"trial:days=60"},
			},
			end: now.Add(asDays(-1)),
			err: nil,
		},
		{
			name: "less than a day left",
			organization: &Organization{
				CreatedAt:    now.Add(asDays(-10) + 1*time.Hour),
				FeatureFlags: []string{"trial:days=10"},
			},
			end: now.Add(1 * time.Hour),
			err: nil,
		},
		{
			name: "no tag",
			organization: &Organization{
				CreatedAt:    now.Add(asDays(-10) + 1*time.Hour),
				FeatureFlags: []string{},
			},
			end: now.Add(asDays(20) + 1*time.Hour),
			err: nil,
		},
		{
			name: "malformed tag",
			organization: &Organization{
				CreatedAt:    now.Add((-10*24)*time.Hour + 1*time.Hour),
				FeatureFlags: []string{"trial:garbage=123"},
			},
			end: now.Add(asDays(20) + 1*time.Hour),
			err: nil,
		},
		{
			name: "non-numeric tag",
			organization: &Organization{
				FeatureFlags: []string{"trial:days=NAN!"},
			},
			end: time.Time{},
			err: fmt.Errorf(`strconv.Atoi: parsing "NAN!": invalid syntax`),
		},
	} {
		gotTrialEnd, gotErr := example.organization.TrialExpiry()
		if fmt.Sprint(gotTrialEnd) != fmt.Sprint(example.end) {
			t.Errorf("[%s]\nExpected end: %#v\n     Got end: %#v", example.name, example.end, gotTrialEnd)
		}
		if fmt.Sprint(example.err) != fmt.Sprint(gotErr) {
			t.Errorf("[%s]\nExpected error: %v\n     Got error: %v", example.name, example.err, gotErr)
		}
	}
}
