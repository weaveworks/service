package aws_test

import (
	"testing"

	"github.com/prometheus/common/model"

	"github.com/stretchr/testify/assert"
	"github.com/weaveworks/service/dashboard-api/aws"
)

func TestToDashboardID(t *testing.T) {
	assert.Equal(t, "aws-rds", aws.RDS.Type.ToDashboardID())
	assert.Equal(t, "aws-sqs", aws.SQS.Type.ToDashboardID())
	assert.Equal(t, "aws-elb", aws.ELB.Type.ToDashboardID())
	assert.Equal(t, "aws-lambda", aws.Lambda.Type.ToDashboardID())
}

func TestDimensionToLabelName(t *testing.T) {
	assert.Equal(t, model.LabelName("dbinstance_identifier"), aws.DBInstanceIdentifier.ToLabelName())
	assert.Equal(t, model.LabelName("queue_name"), aws.QueueName.ToLabelName())
	assert.Equal(t, model.LabelName("load_balancer_name"), aws.LoadBalancerName.ToLabelName())
	assert.Equal(t, model.LabelName("function_name"), aws.FunctionName.ToLabelName())
}
