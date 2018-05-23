package event

import "github.com/weaveworks/service/notification-eventmanager/types"

type ReceiverData interface {
	Type() string
}

type SlackReceiverData struct {
	Channel     string                  `json:"channel,omitempty"`
	Username    string                  `json:"username,omitempty"`
	Text        string                  `json:"text"`
	IconEmoji   string                  `json:"icon_emoji,omitempty"`
	IconURL     string                  `json:"icon_url,omitempty"`
	LinkNames   bool                    `json:"link_names,omitempty"`
	Attachments []types.SlackAttachment `json:"attachments"`
}

func (s SlackReceiverData) Type() string {
	return types.SlackReceiver
}

type BrowserReceiverData struct {
	Text string `json:"text"`
}

func (b BrowserReceiverData) Type() string {
	return types.BrowserReceiver
}
