package publisher

import (
	"time"

	"golang.org/x/net/context"

	"cloud.google.com/go/pubsub"
	log "github.com/sirupsen/logrus"
)

// Publisher wraps around Client, Topic and Subscription abstractions.
type Publisher struct {
	ctx    context.Context
	client *pubsub.Client
	topic  *pubsub.Topic
}

// New is the constructor for new Publisher instances.
func New(ctx context.Context, projectID, topicName string) (*Publisher, error) {
	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		log.Errorf("Failed to create Pub/Sub client for project [%v]: %v", projectID, err)
		return nil, err
	}
	topic, err := mustGetOrCreateTopic(ctx, client, topicName)
	if err != nil {
		log.Errorf("Failed to get or create Pub/Sub topic [%v] for project [%v]: %v", topicName, projectID, err)
		return nil, err
	}
	return &Publisher{
		ctx:    ctx,
		client: client,
		topic:  topic,
	}, nil
}

func mustGetOrCreateTopic(ctx context.Context, client *pubsub.Client, topicName string) (*pubsub.Topic, error) {
	topic := client.Topic(topicName)
	ok, err := topic.Exists(ctx)
	if err != nil {
		log.Errorf("Failed to check topic [%v]'s existence: %v", topicName, err)
		return nil, err
	}
	if !ok {
		topic, err = client.CreateTopic(ctx, topicName)
		if err != nil {
			log.Errorf("Failed to create topic [%v]: %v", topicName, err)
			return nil, err
		}
	}
	return topic, nil
}

// CreateSubscription is a convenience method to create a subscription for this publisher's project and topic.
// It is not required to call this method to publish messages if you expect a subscription to already exist.
func (p Publisher) CreateSubscription(subName, endpoint string, ackDeadline time.Duration) (*pubsub.Subscription, error) {
	return mustGetOrCreateSubscription(p.ctx, p.client, p.topic, subName, endpoint, ackDeadline)
}

func mustGetOrCreateSubscription(ctx context.Context, client *pubsub.Client, topic *pubsub.Topic, subName, endpoint string, ackDeadline time.Duration) (*pubsub.Subscription, error) {
	sub := client.Subscription(subName)
	exists, err := sub.Exists(ctx)
	if err != nil {
		log.Errorf("Failed to check subscription [%v]'s existence: %v", subName, err)
		return nil, err
	}
	if !exists {
		sub, err = client.CreateSubscription(ctx, subName, pubsub.SubscriptionConfig{
			PushConfig:  pubsub.PushConfig{Endpoint: endpoint},
			Topic:       topic,
			AckDeadline: ackDeadline,
		})
		if err != nil {
			log.Errorf("Failed to create subscription [%v] on [%v] with endpoint [%v]: %v", subName, topic.ID(), endpoint, err)
			return nil, err
		}
	}
	return sub, nil
}

// PublishSync send the provided message to this publisher configured project and topic.
// It is synchronous, i.e. blocks until it received confirmation from Google Pub/Sub that the message was received and enqueued.
func (p Publisher) PublishSync(data []byte) (string, error) {
	r := p.topic.Publish(p.ctx, &pubsub.Message{
		Data: data,
	})
	msgID, err := r.Get(p.ctx) // Blocks until Publish succeeds or context is done.
	if err != nil {
		log.Errorf("Failed to publish message [%v]", data)
		return "", err
	}
	return msgID, nil
}

// Close frees resources currently used.
func (p Publisher) Close() {
	p.topic.Stop()
	p.client.Close()
}
