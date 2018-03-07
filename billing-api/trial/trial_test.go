package trial

import (
	"fmt"
	"testing"
	"time"

	"github.com/weaveworks/service/users"
)

func asDays(days int) time.Duration {
	return time.Duration(days*24) * time.Hour
}

func TestTrialInfo(t *testing.T) {
	now := time.Now().UTC()
	for _, example := range []struct {
		name         string
		organization users.Organization
		trial        Trial
	}{
		{
			name: "basic",
			organization: users.Organization{
				CreatedAt:      now.Add(asDays(-55) + 1*time.Hour),
				TrialExpiresAt: now.Add(asDays(5) + 1*time.Hour),
			},
			trial: Trial{
				Length:    60,
				Remaining: 6,
				Start:     now.Add(asDays(-55) + 1*time.Hour),
				End:       now.Add(asDays(5) + 1*time.Hour),
			},
		},
		{
			name: "expired",
			organization: users.Organization{
				CreatedAt:      now.Add(asDays(-61)),
				TrialExpiresAt: now.Add(asDays(-1)),
			},
			trial: Trial{
				Length:    60,
				Remaining: 0,
				Start:     now.Add(asDays(-61)),
				End:       now.Add(asDays(-1)),
			},
		},
		{
			name: "less than a day left",
			organization: users.Organization{
				CreatedAt:      now.Add(asDays(-10) + 1*time.Hour),
				TrialExpiresAt: now.Add(1 * time.Hour),
			},
			trial: Trial{
				Length:    10,
				Remaining: 1,
				Start:     now.Add(asDays(-10) + 1*time.Hour),
				End:       now.Add(1 * time.Hour),
			},
		},
		{
			name: "no tag",
			organization: users.Organization{
				CreatedAt:      now.Add(asDays(-10) + 1*time.Hour),
				TrialExpiresAt: now.Add(asDays(20) + 1*time.Hour),
			},
			trial: Trial{
				Length:    defaultTrialLength,
				Remaining: 21,
				Start:     now.Add(asDays(-10) + 1*time.Hour),
				End:       now.Add(asDays(20) + 1*time.Hour),
			},
		},
		{
			name: "created after trial expired",
			organization: users.Organization{
				CreatedAt:      now.Add(time.Hour),
				TrialExpiresAt: now.Add(-time.Hour),
			},
			trial: Trial{
				Length:    0,
				Remaining: 0,
				Start:     now.Add(-time.Hour),
				End:       now.Add(-time.Hour),
			},
		},
	} {
		gotTrial := Info(example.organization.TrialExpiresAt, example.organization.CreatedAt, now)
		if fmt.Sprint(gotTrial) != fmt.Sprint(example.trial) {
			t.Errorf("[%s]\nExpected trial: %#v\n     Got trial: %#v", example.name, example.trial, gotTrial)
		}
	}
}
