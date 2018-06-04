package aws_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/weaveworks/service/dashboard-api/aws"
)

func TestTypeToDashboardID(t *testing.T) {
	assert.Equal(t, "aws-rds", aws.TypeToDashboardID(aws.RDS))
	assert.Equal(t, "aws-sqs", aws.TypeToDashboardID(aws.SQS))
	assert.Equal(t, "aws-elb", aws.TypeToDashboardID(aws.ELB))
	assert.Equal(t, "aws-lambda", aws.TypeToDashboardID(aws.Lambda))
}
