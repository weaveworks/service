package browser

import "github.com/weaveworks/service/notification-eventmanager/types"

type ReceiverData struct {
	// TODO(rndstr): remove this? it is already available
	EventType string `json:"type"`
	Text      string `json:"text"`
}

func (r ReceiverData) Type() string {
	return types.BrowserReceiver
}
