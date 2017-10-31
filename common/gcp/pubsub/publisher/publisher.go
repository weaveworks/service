package publisher

import (
	"context"
	"io/ioutil"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"

	"github.com/weaveworks/service/common/gcp/pubsub/dto"
)

// Publisher wraps around Client, Topic and Subscription abstractions.
type Publisher struct {
	ctx    context.Context
	client *pubsub.Client
	topic  *pubsub.Topic
}

// New is the constructor for new Publisher instances.
func New(ctx context.Context, cfg Config) (*Publisher, error) {
	// Create source for oauth2 token
	jsonKey, err := ioutil.ReadFile(cfg.serviceAccountKeyFile)
	if err != nil {
		return nil, err
	}
	conf, err := google.JWTConfigFromJSON(jsonKey, pubsub.ScopePubSub, pubsub.ScopeCloudPlatform)
	if err != nil {
		return nil, err
	}
	ts := conf.TokenSource(ctx)

	client, err := pubsub.NewClient(ctx, cfg.projectID, option.WithTokenSource(ts))
	if err != nil {
		return nil, errors.Wrapf(err, "cannot create client for project [%v]", cfg.projectID)
	}
	var topic *pubsub.Topic
	if cfg.CreateTopic {
		topic, err = createTopic(ctx, client, cfg.topicID, cfg.topicProjectID)
	} else {
		topic = newTopic(client, cfg.topicID, cfg.topicProjectID)
	}
	if err != nil {
		return nil, errors.Wrapf(err, "cannot create topic [%v] in project [%v] for project [%v]", cfg.topicID, cfg.topicProjectID, cfg.projectID)
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
	ok, err := topic.Exists(ctx)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot check for topic's existence")
	}
	if !ok {
		topic, err = client.CreateTopic(ctx, topicID)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot create topic")
		}
	}
	return topic, nil
}

// CreateSubscription is a convenience method to create a subscription for this publisher's project and topic.
// It is not required to call this method to publish messages if you expect a subscription to already exist.
func (p Publisher) CreateSubscription(subName, endpoint string, ackDeadline time.Duration) (*pubsub.Subscription, error) {
	return getOrCreateSubscription(p.ctx, p.client, p.topic, subName, endpoint, ackDeadline)
}

func getOrCreateSubscription(ctx context.Context, client *pubsub.Client, topic *pubsub.Topic, subID, endpoint string, ackDeadline time.Duration) (*pubsub.Subscription, error) {
	sub := client.Subscription(subID)
	exists, err := sub.Exists(ctx)
	if err != nil {
		log.Errorf("Failed to check subscription [%v]'s existence: %v", subID, err)
		return nil, err
	}
	if !exists {
		sub, err = client.CreateSubscription(ctx, subID, pubsub.SubscriptionConfig{
			PushConfig:  pubsub.PushConfig{Endpoint: endpoint},
			Topic:       topic,
			AckDeadline: ackDeadline,
		})
		if err != nil {
			return nil, errors.Wrapf(err, "cannot create subscription [%v] on [%v] with endpoint [%v]", subID, topic.ID(), endpoint)
		}
	}
	return sub, nil
}

// PublishSync send the provided message to this publisher configured project and topic.
// It is synchronous, i.e. blocks until it received confirmation from Google Pub/Sub that the message was received and enqueued.
func (p Publisher) PublishSync(msg dto.Message) (string, error) {
	r := p.topic.Publish(p.ctx, &pubsub.Message{
		ID:         msg.MessageID,
		Data:       msg.Data,
		Attributes: msg.Attributes,
	})
	msgID, err := r.Get(p.ctx) // Blocks until Publish succeeds or context is done.
	if err != nil {
		log.Errorf("Failed to publish message [%+v]", msg)
		return "", err
	}
	return msgID, nil
}

// Close frees resources currently used.
func (p Publisher) Close() {
	p.topic.Stop()
	p.client.Close()
}
