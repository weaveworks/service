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

// Dimension describes an AWS resource's metrics dimension, e.g. DBInstanceIdentifier.
type Dimension string

// Supported AWS dimensions:
const (
	// AWS RDS (https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/rds-metricscollected.html#rds-metric-dimensions):
	DBInstanceIdentifier = Dimension("DBInstanceIdentifier")
	// AWS SQS (https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/sqs-metricscollected.html#sqs-metric-dimensions):
	QueueName = Dimension("QueueName")
	// AWS ELB (https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/elb-metricscollected.html#load-balancer-metric-dimensions-clb):
	LoadBalancerName = Dimension("LoadBalancerName")
	// AWS Lambda (https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/lam-metricscollected.html#lam-metric-dimensions):
	FunctionName = Dimension("FunctionName")
)

// TypeToDashboardID converts a supported AWS type to a dashboard ID.
func TypeToDashboardID(awsType Type) string {
	return fmt.Sprintf("aws-%v", awsType)
}
