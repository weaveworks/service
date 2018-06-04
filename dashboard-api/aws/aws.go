package aws

import (
	"fmt"
	"strings"
)

// Supported AWS types:
const (
	RDS    = "RDS"
	SQS    = "SQS"
	ELB    = "ELB"
	Lambda = "Lambda"
)

// TypeToDashboardID converts a supported AWS type to a dashboard ID.
func TypeToDashboardID(awsType string) string {
	return fmt.Sprintf("aws-%v", strings.ToLower(awsType))
}
