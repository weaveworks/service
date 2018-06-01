package main

import (
	"context"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

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

// An array of AWS resources, alphabetically sorted by type and name.
type resourcesByType = map[string][]string

func (api *API) getAWSResources(ctx context.Context) (resourcesByType, error) {
	to := time.Now()
	from := to.Add(-1 * time.Hour) // Not too long in the past so that Cortex serves the series from memcached.
	labelSets, err := api.prometheus.Series(ctx, []string{"{kubernetes_namespace=\"weave\",_weave_service=\"cloudwatch-exporter\"}"}, from, to)
	if err != nil {
		return nil, err
	}
	return labelSetsToResources(labelSets), nil
}

// An AWS resource.
type resource struct {
	Type string `json:"type"` // e.g. ELB, RDS, SQS, etc.
	Name string `json:"name"`
}

func labelSetsToResources(labelSets []model.LabelSet) resourcesByType {
	resourcesSets := make(map[string]map[string]bool)
	for _, labelSet := range labelSets {
		for _, tln := range typesAndLabelNames {
			if resourceName, ok := labelSet[tln.LabelName]; ok {
				if _, ok := resourcesSets[tln.Type]; !ok {
					resourcesSets[tln.Type] = make(map[string]bool)
				}
				resourcesSets[tln.Type][string(resourceName)] = true
				break
			}
		}
	}
	resources := make(resourcesByType, len(resourcesSets))
	for rtype, rset := range resourcesSets {
		array := setToArray(rset)
		sort.Strings(array)
		resources[rtype] = array
	}

	return resources
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

var typesAndLabelNames = awsMetricDimensionsToLabels(awsMetricDimensions)

type typeAndDimension struct {
	Type      string
	Dimension string
}

var awsMetricDimensions = []typeAndDimension{
	// AWS RDS (https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/rds-metricscollected.html#rds-metric-dimensions):
	{Type: "RDS", Dimension: "DBInstanceIdentifier"},

	// AWS SQS (https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/sqs-metricscollected.html#sqs-metric-dimensions):
	{Type: "SQS", Dimension: "QueueName"},

	// AWS ELB (https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/elb-metricscollected.html#load-balancer-metric-dimensions-clb):
	{Type: "ELB", Dimension: "LoadBalancerName"},

	// AWS Lambda (https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/lam-metricscollected.html#lam-metric-dimensions):
	{Type: "Lambda", Dimension: "FunctionName"},
}

type typeAndLabel struct {
	Type      string
	LabelName model.LabelName
}

func awsMetricDimensionsToLabels(typeAndDimensions []typeAndDimension) []typeAndLabel {
	labels := []typeAndLabel{}
	for _, td := range typeAndDimensions {
		labels = append(labels, typeAndLabel{
			Type:      td.Type,
			LabelName: model.LabelName(toSnakeCase(td.Dimension)),
		})
	}
	return labels
}

// Use the similar conversion from AWS CloudWatch dimensions to Prometheus labels as the CloudWatch exporter.
// See also:
// - https://github.com/prometheus/cloudwatch_exporter/blob/cloudwatch_exporter-0.1.0/src/main/java/io/prometheus/cloudwatch/CloudWatchCollector.java#L311-L313
// - https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/CW_Support_For_AWS.html

var snakeCaseRegexp = regexp.MustCompile(`([a-z0-9])([A-Z])`)

func toSnakeCase(s string) string {
	return strings.ToLower(snakeCaseRegexp.ReplaceAllString(s, `${1}_${2}`))
}
