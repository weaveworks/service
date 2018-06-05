package aws

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/prometheus/common/model"
)

// Namespace is the namespace of Weaveworks' AWS CloudWatcher pod.
const Namespace = "weave"

// Service is the name of Weaveworks' AWS CloudWatcher pod.
const Service = "cloudwatch-exporter"

// Product represents an AWS product.
type Product struct {
	Type          Type
	Category      Category
	NameDimension Dimension       // The CloudWatch metrics dimension identifying an instance of this product.
	LabelName     model.LabelName // The Prometheus label name corresponding to NameDimension.
}

// Products represents the list of supported AWS products.
// N.B.: the order of the below products matters, and corresponds to:
// - to the order of the elements returned by the /api/dashboard/aws/resources endpoint, and therefore
// - to the order in which these should be rendered in the frontend.
var Products = []Product{
	{Type: RDS, Category: Database, NameDimension: DBInstanceIdentifier, LabelName: DBInstanceIdentifier.ToLabelName()},
	{Type: SQS, Category: Queue, NameDimension: QueueName, LabelName: QueueName.ToLabelName()},
	{Type: ELB, Category: LoadBalancer, NameDimension: LoadBalancerName, LabelName: LoadBalancerName.ToLabelName()},
	{Type: Lambda, Category: LambdaFunction, NameDimension: FunctionName, LabelName: FunctionName.ToLabelName()},
}

// Type describes an AWS resource type, e.g. rds.
type Type string

// ToDashboardID converts this AWS resource type to a dashboard ID.
func (t Type) ToDashboardID() string {
	return fmt.Sprintf("aws-%v", t)
}

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

// ToLabelName converts this AWS metrics dimension to a Prometheus label name.
func (d Dimension) ToLabelName() model.LabelName {
	return model.LabelName(toSnakeCase(string(d)))
}

// Use the similar conversion from AWS CloudWatch dimensions to Prometheus labels as the CloudWatch exporter.
// See also:
// - https://github.com/prometheus/cloudwatch_exporter/blob/cloudwatch_exporter-0.1.0/src/main/java/io/prometheus/cloudwatch/CloudWatchCollector.java#L311-L313
// - https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/CW_Support_For_AWS.html

var snakeCaseRegexp = regexp.MustCompile(`([a-z0-9])([A-Z])`)

func toSnakeCase(s string) string {
	return strings.ToLower(snakeCaseRegexp.ReplaceAllString(s, `${1}_${2}`))
}
