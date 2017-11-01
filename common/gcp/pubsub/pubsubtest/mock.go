// +build !integration

package pubsubtest

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"cloud.google.com/go/pubsub"

	"github.com/weaveworks/service/common/gcp/pubsub/dto"
	"github.com/weaveworks/service/common/gcp/pubsub/publisher"
)

// Setup creates a mocked publisher.
func Setup(_ context.Context, _ *testing.T, _ publisher.Config) publisher.Interface {
	return &mockPublisher{}
}

// MockPublisher implements publisher.Interface
type mockPublisher struct {
	endpoint   string
	httpClient *http.Client
}

// CreateSubscription sets up the publisher and returns an empty subscription.
func (p *mockPublisher) CreateSubscription(subID, endpoint string, ackDeadline time.Duration) (*pubsub.Subscription, error) {
	p.httpClient = &http.Client{Timeout: 2 * time.Second}
	p.endpoint = endpoint
	return &pubsub.Subscription{}, nil
}

// PublishSync circumvents any PubSub server by directly calling the configured push endpoint.
func (p *mockPublisher) PublishSync(data []byte, attrs map[string]string) (string, error) {
	if p.endpoint == "" {
		panic("empty endpoint, call CreateSubscription() first")
	}
	ev := dto.Event{
		Subscription: "project/p/subscription/s",
		Message: dto.Message{
			Data:       data,
			Attributes: attrs,
		},
	}
	bs, err := json.Marshal(ev)
	if err != nil {
		return "", err
	}
	_, err = p.httpClient.Post(p.endpoint, "application/json", bytes.NewBuffer(bs))
	if err != nil {
		return "", err
	}

	return "6", nil
}

// Close doesn't do anything.
func (p *mockPublisher) Close() {
}
