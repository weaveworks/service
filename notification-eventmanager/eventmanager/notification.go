package eventmanager

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"

	"github.com/weaveworks/service/notification-eventmanager/types"
	"github.com/weaveworks/service/notification-sender"
)

const batchSize = 10

// Wait waits until all SQS send requests are finished
func (em *EventManager) Wait() {
	em.wg.Wait()
}

// sendNotificationBatchesToQueue gets notifications for event, partitions them to batches and sends to SQS queue
func (em *EventManager) sendNotificationBatchesToQueue(ctx context.Context, e types.Event) error {
	notifs, err := em.getNotifications(ctx, e)
	if err != nil {
		return errors.Wrapf(err, "cannot get all notifications for event %v", e)
	}

	notifBatches := partitionNotifications(notifs, batchSize)
	for _, batch := range notifBatches {
		sendInp, err := em.notificationBatchToSendInput(batch)
		if err != nil {
			return errors.Wrap(err, "cannot get SQS send input for notification batch")
		}

		em.wg.Add(1)
		go func() {
			defer em.wg.Done()
			_, err = em.SQSClient.SendMessageBatch(sendInp)
			if err != nil {
				log.Errorf("cannot send to SQS queue batch input, error: %s", err)
				eventsToSQSError.With(prometheus.Labels{"event_type": e.Type}).Inc()
				return
			}
			sender.NotificationsInSQS.Add(float64(len(notifs)))
		}()
	}

	return nil
}

func (em *EventManager) getNotifications(ctx context.Context, e types.Event) ([]types.Notification, error) {
	receivers, err := em.DB.GetReceiversForEvent(e)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot get receivers for event %v", e)
	}
	log.Debugf("Got %d receivers for InstanceID = %s and event type = %s", len(receivers), e.InstanceID, e.Type)

	var notifications []types.Notification
	for _, r := range receivers {
		notif := types.Notification{
			ReceiverType: r.RType,
			InstanceID:   e.InstanceID,
			Address:      r.AddressData,
			Data:         renderData(&e, r.RType),
			Event:        e,
		}
		notifications = append(notifications, notif)
	}

	return notifications, nil
}

// partitionNotifications takes slice of notifications, partitions it to batches with size batchSize
// and returns slice of slices of notifications
func partitionNotifications(notifs []types.Notification, batchSize int) [][]types.Notification {
	var batch []types.Notification
	var notifBatches [][]types.Notification

	for len(notifs) >= batchSize {
		batch, notifs = notifs[:batchSize], notifs[batchSize:]
		notifBatches = append(notifBatches, batch)
	}

	if len(notifs) > 0 {
		notifBatches = append(notifBatches, notifs)
	}

	return notifBatches
}

func (em *EventManager) notificationBatchToSendInput(batch []types.Notification) (*sqs.SendMessageBatchInput, error) {
	var entries []*sqs.SendMessageBatchRequestEntry
	for i, notif := range batch {
		notifBytes, err := json.Marshal(notif)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot marshal notification %s", notif)
		}
		entry := &sqs.SendMessageBatchRequestEntry{
			Id:          aws.String(strconv.Itoa(i)),
			MessageBody: aws.String(string(notifBytes)),
		}
		entries = append(entries, entry)
	}
	return &sqs.SendMessageBatchInput{
		Entries:  entries,
		QueueUrl: &em.SQSQueue,
	}, nil
}
