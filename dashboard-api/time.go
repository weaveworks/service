package main

import (
	"fmt"
	"math"
	"strconv"
	"time"
)

// From github.com/prometheus/prometheus/web/api/v1/api.go.
// We forward the start/end query parameters to prometheus, this makes sure the
// same code parses those values.
func parseTime(s string) (time.Time, error) {
	if t, err := strconv.ParseFloat(s, 64); err == nil {
		s, ns := math.Modf(t)
		return time.Unix(int64(s), int64(ns*float64(time.Second))), nil
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("cannot parse %q to a valid timestamp", s)
}
