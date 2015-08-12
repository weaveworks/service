package main

import (
	"time"

	"github.com/weaveworks/service/users/names"
)

type organization struct {
	ID                 string
	Name               string
	ProbeToken         string
	FirstProbeUpdateAt time.Time
	CreatedAt          time.Time
}

func (o *organization) RegenerateName() {
	o.Name = names.Generate()
}

func (o *organization) RegenerateProbeToken() error {
	t, err := generateToken()
	if err != nil {
		return err
	}
	o.ProbeToken = t
	return nil
}
