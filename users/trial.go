package users

import (
	"strconv"
	"strings"
	"time"
)

// TODO: This is copied & modified from weaveworks/billing. We should
// eliminate the duplication.

// TODO: Rather than storing the trial period as a number of days in a feature
// flag, instead store a concrete trial expiry date.

const (
	trialFlag          string = "trial:days"
	defaultTrialLength int    = 30
)

// CalculateTrialExpiry returns the date when an organization's free trial
// period expires.
func CalculateTrialExpiry(createdAt time.Time, featureFlags []string) (time.Time, error) {
	length, err := parseTrialLength(featureFlags)
	if err != nil {
		return time.Time{}, err
	}
	return createdAt.UTC().Add(length), nil
}

func parseTrialLength(featureFlags []string) (time.Duration, error) {
	var days = defaultTrialLength
	for _, rawFlag := range featureFlags {
		flag, value := parseMachineFlag(rawFlag)
		if flag == trialFlag {
			var err error
			days, err = strconv.Atoi(value)
			if err != nil {
				return 0, err
			}
		}
	}
	return time.Duration(days*24) * time.Hour, nil
}

func parseMachineFlag(flag string) (string, string) {
	s := strings.Split(flag, "=")
	if len(s) == 2 {
		return s[0], s[1]
	}
	return "", ""
}

func days(duration time.Duration) float64 {
	return duration.Hours() / 24.0
}
