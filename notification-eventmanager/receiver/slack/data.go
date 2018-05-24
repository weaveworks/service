package slack

import "github.com/weaveworks/service/notification-eventmanager/types"

type ReceiverData struct {
	Channel     string                  `json:"channel,omitempty"`
	Username    string                  `json:"username,omitempty"`
	Text        string                  `json:"text"`
	IconEmoji   string                  `json:"icon_emoji,omitempty"`
	IconURL     string                  `json:"icon_url,omitempty"`
	LinkNames   bool                    `json:"link_names,omitempty"`
	Attachments []types.SlackAttachment `json:"attachments"`
}

func (r ReceiverData) Type() string {
	return types.SlackReceiver
}
