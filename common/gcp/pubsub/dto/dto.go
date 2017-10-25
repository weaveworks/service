package dto

import (
	"encoding/base64"
	"encoding/json"
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

// Unmarshal deserialises the provided bytes into this Event.
func (e *Event) Unmarshal(data []byte) error {
	return json.Unmarshal(data, e)
}

// Message is a nested struct within Event.
type Message struct {
	Data       string            `json:"data"`
	MessageID  string            `json:"messageId"`
	Attributes map[string]string `json:"attributes"`
	Bytes      []byte
}

// Decode base64-decodes this message's Data field in this message's Bytes field.
func (m *Message) Decode() error {
	decoded, err := base64.StdEncoding.DecodeString(m.Data)
	m.Bytes = decoded
	return err
}
