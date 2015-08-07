package main

import "time"

type Organization struct {
	ID                 string
	Name               string
	ProbeToken         string
	FirstProbeUpdateAt time.Time
	CreatedAt          time.Time
}

func generateProbeToken() (string, error) {
	return secureRandomBase64(20)
}
