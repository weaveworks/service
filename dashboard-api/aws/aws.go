package aws

import (
	"fmt"
)

// Namespace is the namespace of Weaveworks' AWS CloudWatcher pod.
const Namespace = "weave"

// Service is the name of Weaveworks' AWS CloudWatcher pod.
const Service = "cloudwatch-exporter"

// Type describes an AWS resource type, e.g. RDS.
type Type string

// Supported AWS types:
const (
	RDS    = Type("rds")
	SQS    = Type("sqs")
	ELB    = Type("elb")
	Lambda = Type("lambda")
)

// TypeToDashboardID converts a supported AWS type to a dashboard ID.
func TypeToDashboardID(awsType Type) string {
	return fmt.Sprintf("aws-%v", awsType)
}
