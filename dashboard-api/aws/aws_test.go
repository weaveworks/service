package aws_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/weaveworks/service/dashboard-api/aws"
)

func TestTypeToDashboardID(t *testing.T) {
	assert.Equal(t, "aws-rds", aws.RDS.ToDashboardID())
	assert.Equal(t, "aws-sqs", aws.SQS.ToDashboardID())
	assert.Equal(t, "aws-elb", aws.ELB.ToDashboardID())
	assert.Equal(t, "aws-lambda", aws.Lambda.ToDashboardID())
}
