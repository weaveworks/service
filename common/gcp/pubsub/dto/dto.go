package dto

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
}
