package main

import (
	"time"

	"github.com/weaveworks/service/users/names"
)

type Organization struct {
	ID                 string
	Name               string
	ProbeToken         string
	FirstProbeUpdateAt time.Time
	CreatedAt          time.Time
}

func (o *Organization) RegenerateName() {
	o.Name = names.Generate()
}

func (o *Organization) RegenerateProbeToken() error {
	t, err := secureRandomBase64(20)
	if err != nil {
		return err
	}
	o.ProbeToken = t
	return nil
}
