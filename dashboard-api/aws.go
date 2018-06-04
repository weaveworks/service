package main

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/weaveworks/service/dashboard-api/aws"

	"github.com/prometheus/common/model"
	log "github.com/sirupsen/logrus"
	"github.com/weaveworks/common/user"
	"github.com/weaveworks/service/common/render"
)

// GetAWSResources returns the list of AWS-managed services from CloudWatch-exported metrics.
func (api *API) GetAWSResources(w http.ResponseWriter, r *http.Request) {
	orgID, ctx, err := user.ExtractOrgIDFromHTTPRequest(r)
	ctx, cancel := context.WithTimeout(ctx, api.cfg.prometheus.timeout)
	defer cancel()

	log.WithField("orgID", orgID).WithField("url", r.URL).Debug("GetAWSResources")

	resources, err := api.getAWSResources(ctx)
	if err != nil {
		renderError(w, r, err)
		return
	}
	render.JSON(w, http.StatusOK, resources)
}

func (api *API) getAWSResources(ctx context.Context) ([]resources, error) {
	to := time.Now()
	from := to.Add(-1 * time.Hour) // Not too long in the past so that Cortex serves the series from memcached.
	labelSets, err := api.prometheus.Series(ctx, []string{fmt.Sprintf("{kubernetes_namespace=\"%v\",_weave_service=\"%v\"}", aws.Namespace, aws.Service)}, from, to)
	if err != nil {
		return nil, err
	}
	return labelSetsToResources(labelSets), nil
}

// An AWS resource.
type resource struct {
	Type aws.Type `json:"type"` // e.g. ELB, RDS, SQS, etc.
	Name string   `json:"name"`
}

// AWS resources,
// - grouped by type,
// - in the order we want these to be rendered,
// - with names sorted alphabetically.
type resources struct {
	Type     aws.Type `json:"type"` // e.g. ELB, RDS, SQS, etc.
	Category string   `json:"category"`
	Names    []string `json:"names"`
}

func labelSetsToResources(labelSets []model.LabelSet) []resources {
	resourcesSets := make(map[aws.Type]map[string]bool)
	for _, labelSet := range labelSets {
		for rtype, labelName := range typesToLabelNames {
			if resourceName, ok := labelSet[labelName]; ok {
				if _, ok := resourcesSets[rtype]; !ok {
					resourcesSets[rtype] = make(map[string]bool)
				}
				resourcesSets[rtype][string(resourceName)] = true
				break
			}
		}
	}
	resourcesArray := []resources{}
	for _, rtype := range types {
		if rset, ok := resourcesSets[rtype]; ok {
			array := setToArray(rset)
			sort.Strings(array)
			resourcesArray = append(resourcesArray, resources{
				Type:     rtype,
				Category: categories[rtype],
				Names:    array,
			})
		}
	}

	return resourcesArray
}

func setToArray(set map[string]bool) []string {
	array := make([]string, len(set))
	i := 0
	for elem := range set {
		array[i] = elem
		i++
	}
	return array
}

// e.g.: "RDS" -> "dbinstance_identifier" -- see tests.
var typesToLabelNames = awsMetricDimensionsToLabelNames(awsMetricDimensions)

type typeAndDimension struct {
	Type      aws.Type
	Category  string
	Dimension string
}

const (
	database       = "Database"
	loadBalancer   = "Load Balancer"
	queue          = "Queue"
	lambdaFunction = "λ-Function"
)

// N.B.: the order of the below types corresponds to the ordering of resources in the payload, and therefore, how these should be rendered in the frontend.
var awsMetricDimensions = []typeAndDimension{
	// AWS RDS (https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/rds-metricscollected.html#rds-metric-dimensions):
	{Type: aws.RDS, Category: database, Dimension: "DBInstanceIdentifier"},

	// AWS SQS (https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/sqs-metricscollected.html#sqs-metric-dimensions):
	{Type: aws.SQS, Category: queue, Dimension: "QueueName"},

	// AWS ELB (https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/elb-metricscollected.html#load-balancer-metric-dimensions-clb):
	{Type: aws.ELB, Category: loadBalancer, Dimension: "LoadBalancerName"},

	// AWS Lambda (https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/lam-metricscollected.html#lam-metric-dimensions):
	{Type: aws.Lambda, Category: lambdaFunction, Dimension: "FunctionName"},
}

var types = awsMetricDimentionsToTypes(awsMetricDimensions)
var categories = awsMetricDimentionsToCategories(awsMetricDimensions)

type typeAndLabel struct {
	Type      string
	LabelName model.LabelName
}

func awsMetricDimensionsToLabelNames(typeAndDimensions []typeAndDimension) map[aws.Type]model.LabelName {
	typesToLabelNames := make(map[aws.Type]model.LabelName, len(typeAndDimensions))
	for _, td := range typeAndDimensions {
		typesToLabelNames[td.Type] = model.LabelName(toSnakeCase(td.Dimension))
	}
	return typesToLabelNames
}

func awsMetricDimentionsToTypes(typeAndDimensions []typeAndDimension) []aws.Type {
	types := []aws.Type{}
	for _, td := range typeAndDimensions {
		types = append(types, td.Type)
	}
	return types
}

func awsMetricDimentionsToCategories(typeAndDimensions []typeAndDimension) map[aws.Type]string {
	categories := make(map[aws.Type]string, len(typeAndDimensions))
	for _, td := range typeAndDimensions {
		categories[td.Type] = td.Category
	}
	return categories
}

// Use the similar conversion from AWS CloudWatch dimensions to Prometheus labels as the CloudWatch exporter.
// See also:
// - https://github.com/prometheus/cloudwatch_exporter/blob/cloudwatch_exporter-0.1.0/src/main/java/io/prometheus/cloudwatch/CloudWatchCollector.java#L311-L313
// - https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/CW_Support_For_AWS.html

var snakeCaseRegexp = regexp.MustCompile(`([a-z0-9])([A-Z])`)

func toSnakeCase(s string) string {
	return strings.ToLower(snakeCaseRegexp.ReplaceAllString(s, `${1}_${2}`))
}
