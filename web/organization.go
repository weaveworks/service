package main

import "time"

type Organization struct {
	ID                 string
	Name               string
	FirstProbeUpdateAt time.Time
	LastProbeUpdateAt  time.Time
}
