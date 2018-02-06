package sqsconnect

import (
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/weaveworks/service/notification-configmanager/utils"
)

// Timeout waiting for SQS queue to be created
const timeout = 5 * time.Minute

// NewSQS returns a new instance of the SQS client with a session, queue name and error
func NewSQS(urlString string) (sqsCli *sqs.SQS, queueURL string, err error) {
	sqsConfig, name, err := utils.AWSConfigFromURLString(urlString)
	if err != nil {
		return nil, "", errors.Wrapf(err, "error getting AWS config from URL %s", urlString)
	}

	sess := session.Must(session.NewSession(sqsConfig))
	sqsCli = sqs.New(sess)

	qURL, err := waitForQueue(sqsCli, name)
	if err != nil {
		return nil, "", errors.Wrap(err, "waiting for sqs connection")
	}

	return sqsCli, qURL, nil
}

func waitForQueue(sqsCli *sqs.SQS, prefix string) (queueURL string, err error) {
	deadline := time.Now().Add(timeout)
	for tries := 0; time.Now().Before(deadline); tries++ {
		result, err := sqsCli.CreateQueue(&sqs.CreateQueueInput{
			QueueName: aws.String(prefix),
		})
		if err == nil {
			return *result.QueueUrl, nil
		}
		log.Debugf("queue not created, error: %s; retrying...", err)
		time.Sleep(time.Second << uint(tries))
	}

	return "", errors.Errorf("queue %s not created after %s", prefix, timeout)
}
