package common

import (
	"strings"
)

type ArrayFlags []string

func (a *ArrayFlags) String() string {
	return strings.Join(*a, ",")
}

func (a *ArrayFlags) Set(value string) error {
	*a = append(*a, value)
	return nil
}
