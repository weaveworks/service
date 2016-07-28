package main

import (
	"regexp"
	"time"
)

var (
	orgExternalIDRegex = regexp.MustCompile(`\A[a-zA-Z0-9_-]+\z`)
)

type organization struct {
	ID                 string
	ExternalID         string
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
	case o.ExternalID == "":
		return errOrgExternalIDCannotBeBlank
	case !orgExternalIDRegex.MatchString(o.ExternalID):
		return errOrgExternalIDFormat
	case o.Label == "":
		return errOrgLabelCannotBeBlank
	}
	return nil
}
