package main

import (
	"regexp"
	"time"
)

var (
	orgNameRegex = regexp.MustCompile(`\A[a-zA-Z0-9_-]+\z`)
)

type organization struct {
	ID                 string
	Name               string
	Label              string
	ProbeToken         string
	FirstProbeUpdateAt time.Time
	CreatedAt          time.Time
}

func (o *organization) RegenerateProbeToken() error {
	t, err := generateToken()
	if err != nil {
		return err
	}
	o.ProbeToken = t
	return nil
}

func (o *organization) valid() error {
	switch {
	case o.Name == "":
		return errOrgNameCannotBeBlank
	case !orgNameRegex.MatchString(o.Name):
		return errOrgNameFormat
	case o.Label == "":
		return errOrgLabelCannotBeBlank
	}
	return nil
}
