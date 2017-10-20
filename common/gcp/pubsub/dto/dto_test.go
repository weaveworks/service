package dto_test

import (
	"testing"

	"github.com/weaveworks/service/common/gcp/pubsub/dto"

	"github.com/stretchr/testify/assert"
)

func TestDeserialiseAndDecodeEvent(t *testing.T) {
	bytes := []byte(`{
		"subscription": "projects\/foobar\/subscriptions\/push-https-example",
		"message": {
			"messageId": "1",
			"data": "Zm9vYmFy",
			"attributes": {}
		}
	}`)
	event := dto.Event{}
	err := event.Unmarshal(bytes)
	assert.Nil(t, err)
	assert.Equal(t, "projects/foobar/subscriptions/push-https-example", event.Subscription)
	assert.Equal(t, "1", event.Message.MessageID)
	assert.Equal(t, "Zm9vYmFy", event.Message.Data)
	assert.Equal(t, make(map[string]string), event.Message.Attributes)
	assert.Nil(t, event.Message.Bytes)

	err = event.Message.Decode()
	assert.Nil(t, err)
	assert.Equal(t, "foobar", string(event.Message.Bytes))
}
