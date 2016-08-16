package common

import (
	"strings"
)

// ArrayFlags allows you to collect repeated flags
type ArrayFlags []string

func (a *ArrayFlags) String() string {
	return strings.Join(*a, ",")
}

// Set implements flags.Value
func (a *ArrayFlags) Set(value string) error {
	*a = append(*a, value)
	return nil
}
