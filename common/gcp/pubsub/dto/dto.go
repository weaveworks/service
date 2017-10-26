package dto

import (
	"encoding/base64"
)

// Event is the struct corresponding to the events sent by Google PubSub.
// Example:
//   {
//     "subscription": "projects\/foobar\/subscriptions\/push-https-example",
//     "message": {
//       "messageId": "1",
//       "data": "Zm9vYmFy",
//       "attributes": {}
//     }
//   }
type Event struct {
	Subscription string  `json:"subscription"`
	Message      Message `json:"message"`
}

// Message is a nested struct within Event.
type Message struct {
	Data        []byte            `json:"data"`
	MessageID   string            `json:"messageId"`
	Attributes  map[string]string `json:"attributes"`
	DecodedData []byte            `json:"-"`
}

// Decode base64-decodes this message's Data field in this message's DecodedData field.
func (m *Message) Decode() error {
	_, err := base64.StdEncoding.Decode(m.DecodedData, m.Data)
	return err
}
