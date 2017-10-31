package dto_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/weaveworks/service/common/gcp/pubsub/dto"
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
	err := json.Unmarshal(bytes, &event)
	assert.Nil(t, err)
	assert.Equal(t, "projects/foobar/subscriptions/push-https-example", event.Subscription)
	assert.Equal(t, "1", event.Message.MessageID)
	assert.Equal(t, "Zm9vYmFy", event.Message.Data)
	assert.Equal(t, make(map[string]string), event.Message.Attributes)
	assert.Nil(t, event.Message.DecodedData)

	err = event.Message.Decode()
	assert.Nil(t, err)
	assert.Equal(t, "foobar", string(event.Message.DecodedData))
}
