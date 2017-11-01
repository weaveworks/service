// +build integration

package publisher_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"

	"github.com/weaveworks/service/common/gcp/pubsub/dto"
	"github.com/weaveworks/service/common/gcp/pubsub/publisher"
	"github.com/weaveworks/service/common/gcp/pubsub/pubsubtest"
	"github.com/weaveworks/service/common/gcp/pubsub/webhook"
)

const (
	projectID   = "foo"
	topicID     = "bar"
	subID       = "baz"
	port        = 1337
	ackDeadline = 10 * time.Second
)

var cfg = publisher.Config{
	ProjectID:   projectID,
	TopicID:     topicID,
	CreateTopic: true,
}

func TestPublisher(t *testing.T) {
	pub := pubsubtest.Setup(context.TODO(), t, cfg)
	defer pub.Close()

	// Configure and start the webhook's HTTP server:
	OK := make(chan dto.Message, 1)
	KO := make(chan dto.Message, 1)
	go http.ListenAndServe(
		fmt.Sprintf(":%v", port),
		webhook.New(&testMessageHandler{OK: OK, KO: KO}),
	)

	// Create "push" subscription to redirect messages to our webhook HTTP server:
	endpoint := fmt.Sprintf("http://127.0.0.1:%d", port)
	_, err := pub.CreateSubscription(subID, endpoint, ackDeadline)
	assert.NoError(t, err)
	// Note that we don't sub.Delete() here because the mock implementation will panic.
	// For the emulator, if it isn't restarted between runs, the same subscription will just
	// be picked up again.

	// Send a message and ensure it was processed properly:
	{
		data := []byte("OK")
		attrs := map[string]string{"consumerId": "123"}
		_, err = pub.PublishSync(data, attrs)
		assert.NoError(t, err)

		m := <-OK
		assert.Equal(t, data, m.Data)
		assert.Equal(t, attrs, m.Attributes)
	}

	// Trigger error in handler
	{
		data := []byte("KO")
		_, err = pub.PublishSync(data, nil)
		assert.NoError(t, err) // publishing succeeds
		<-KO                   // handler will detect it as error
	}
}

type testMessageHandler struct {
	OK chan dto.Message
	KO chan dto.Message
}

func (h testMessageHandler) Handle(msg dto.Message) error {
	if string(msg.Data) == "OK" {
		h.OK <- msg
		return nil
	}
	h.KO <- msg
	return fmt.Errorf("invalid data: %s", string(msg.Data))
}
