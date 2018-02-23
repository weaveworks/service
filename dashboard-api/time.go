package main

import (
	"fmt"
	"math"
	"net/http"
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

func parseRequestTime(r *http.Request, param string, defaultValue time.Time) (time.Time, error) {
	value := r.FormValue(param)
	if value == "" {
		return defaultValue, nil
	}

	time, err := parseTime(value)
	if err != nil {
		return defaultValue, err
	}

	return time, nil
}

func parseRequestStartEnd(r *http.Request) (time.Time, time.Time, error) {
	start, err := parseRequestTime(r, "start", time.Now().Add(-time.Hour))
	if err != nil {
		return time.Time{}, time.Time{}, err
	}

	end, err := parseRequestTime(r, "end", time.Now())
	if err != nil {
		return time.Time{}, time.Time{}, err
	}

	return start, end, nil
}
