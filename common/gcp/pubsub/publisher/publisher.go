package publisher

import (
	"context"
	"io/ioutil"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/pkg/errors"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"

	"github.com/weaveworks/service/common/gcp/pubsub/dto"
)

// Interface describes a publisher interacting with PubSub.
type Interface interface {
	CreateSubscription(subName, endpoint string, ackDeadline time.Duration) (*pubsub.Subscription, error)
	PublishSync(data []byte, attrs map[string]string) (string, error)
	Close()
}

// Publisher wraps around Client, Topic and Subscription abstractions.
type Publisher struct {
	ctx    context.Context
	client *pubsub.Client
	topic  *pubsub.Topic
}

// New is the constructor for new Publisher instances.
func New(ctx context.Context, cfg Config) (*Publisher, error) {
	var opts []option.ClientOption
	if cfg.ServiceAccountKeyFile != "" {
		// Create source for oauth2 token
		jsonKey, err := ioutil.ReadFile(cfg.ServiceAccountKeyFile)
		if err != nil {
			return nil, err
		}
		conf, err := google.JWTConfigFromJSON(jsonKey, pubsub.ScopePubSub, pubsub.ScopeCloudPlatform)
		if err != nil {
			return nil, err
		}
		opts = []option.ClientOption{option.WithTokenSource(conf.TokenSource(ctx))}
	}

	client, err := pubsub.NewClient(ctx, cfg.ProjectID, opts...)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot create client for project [%v]", cfg.ProjectID)
	}
	var topic *pubsub.Topic
	if cfg.CreateTopic {
		topic, err = createTopic(ctx, client, cfg.TopicID, cfg.TopicProjectID)
	} else {
		topic = newTopic(client, cfg.TopicID, cfg.TopicProjectID)
	}
	if err != nil {
		return nil, errors.Wrapf(err, "cannot create topic [%v] in project [%v] for project [%v]", cfg.TopicID, cfg.TopicProjectID, cfg.ProjectID)
	}
	return &Publisher{
		ctx:    ctx,
		client: client,
		topic:  topic,
	}, nil
}

// newTopic creates a topic.
func newTopic(client *pubsub.Client, topicID, topicProjectID string) *pubsub.Topic {
	if topicProjectID != "" {
		return client.TopicInProject(topicID, topicProjectID)
	}
	return client.Topic(topicID)
}

// createTopic makes sure the topic exists.
func createTopic(ctx context.Context, client *pubsub.Client, topicID, topicProjectID string) (*pubsub.Topic, error) {
	topic := newTopic(client, topicID, topicProjectID)
	exists, err := topic.Exists(ctx)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot check for topic's existence")
	}
	if !exists {
		topic, err = client.CreateTopic(ctx, topicID)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot create topic")
		}
	}
	return topic, nil
}

// CreateSubscription is a convenience method to create a subscription
// for this publisher's project and topic. It is not required to call
// this method to publish messages if you expect a subscription to already
// exist. If endpoint is empty, creates a pull-based subscription.
func (p Publisher) CreateSubscription(subName, endpoint string, ackDeadline time.Duration) (*pubsub.Subscription, error) {
	sub := p.client.Subscription(subName)
	exists, err := sub.Exists(p.ctx)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot check for existence of subscription [%v]", subName)
	}
	// If it already exists, we delete it to make sure configuration changes propagate
	if exists {
		sub.Delete(p.ctx)
	}
	config := pubsub.SubscriptionConfig{
		Topic:       p.topic,
		AckDeadline: ackDeadline,
	}
	if endpoint != "" {
		config.PushConfig = pubsub.PushConfig{Endpoint: endpoint}
	}
	sub, err = p.client.CreateSubscription(p.ctx, subName, config)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot create subscription [%v] on [%v] with endpoint [%v]", subName, p.topic.ID(), endpoint)
	}
	return sub, nil
}

// ReceiveSubscription sets up a pull subscription with callback.
// Note that this is blocking.
func (p Publisher) ReceiveSubscription(subName string, create bool, ackDeadline time.Duration, callback func(msg dto.Message) error) error {
	sub := p.client.Subscription(subName)
	if create {
		exists, err := sub.Exists(p.ctx)
		if err != nil {
			return errors.Wrapf(err, "cannot check for existence of subscription [%v]", subName)
		}
		// If it already exists, we delete it to make sure configuration changes propagate
		if exists {
			sub.Delete(p.ctx)
		}
		sub, err = p.client.CreateSubscription(p.ctx, subName, pubsub.SubscriptionConfig{
			Topic:       p.topic,
			AckDeadline: ackDeadline,
		})
		if err != nil {
			return errors.Wrapf(err, "cannot create subscription [%v] on [%v]", subName, p.topic.ID())
		}
	}
	err := sub.Receive(p.ctx, func(ctx context.Context, msg *pubsub.Message) {
		hmsg := dto.Message{
			MessageID:  msg.ID,
			Data:       msg.Data,
			Attributes: msg.Attributes,
		}
		if err := callback(hmsg); err != nil {
			msg.Nack()
		} else {
			msg.Ack()
		}
	})
	if err != nil {
		return errors.Wrapf(err, "while receiving from subscription [%v] on [%v]", subName, p.topic.ID())
	}
	return nil
}

// PublishSync sends the data to this publisher configured project and topic.
// It is synchronous, i.e. blocks until it received confirmation from Google Pub/Sub that the message was received and enqueued.
func (p Publisher) PublishSync(data []byte, attrs map[string]string) (string, error) {
	msg := &pubsub.Message{
		Data:       data,
		Attributes: attrs,
	}
	r := p.topic.Publish(p.ctx, msg)
	msgID, err := r.Get(p.ctx) // Blocks until Publish succeeds or context is done.
	if err != nil {
		return "", errors.Wrapf(err, "cannot publish message [%+v]", msg)
	}
	return msgID, nil
}

// Close frees resources currently used.
func (p Publisher) Close() {
	p.topic.Stop()
	p.client.Close()
}
