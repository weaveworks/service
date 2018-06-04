package aws

import (
	"fmt"
	"strings"
)

// Namespace is the namespace of Weaveworks' AWS CloudWatcher pod.
const Namespace = "weave"

// Service is the name of Weaveworks' AWS CloudWatcher pod.
const Service = "cloudwatch-exporter"

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
