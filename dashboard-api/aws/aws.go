package aws

import (
	"fmt"
)

// Namespace is the namespace of Weaveworks' AWS CloudWatcher pod.
const Namespace = "weave"

// Service is the name of Weaveworks' AWS CloudWatcher pod.
const Service = "cloudwatch-exporter"

// Type describes an AWS resource type, e.g. rds.
type Type string

// Supported AWS types:
const (
	RDS    = Type("rds")
	SQS    = Type("sqs")
	ELB    = Type("elb")
	Lambda = Type("lambda")
)

// Category describes AWS types' grouping, e.g. RDS and DynamoDB are both databases.
type Category string

// Supported AWS categories:
const (
	Database       = Category("Database")
	LoadBalancer   = Category("Load Balancer")
	Queue          = Category("Queue")
	LambdaFunction = Category("λ-Function")
)

// TypeToDashboardID converts a supported AWS type to a dashboard ID.
func TypeToDashboardID(awsType Type) string {
	return fmt.Sprintf("aws-%v", awsType)
}
