package email

import "github.com/weaveworks/service/notification-eventmanager/types"

type ReceiverData struct {
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

func (r ReceiverData) Type() string {
	return types.EmailReceiver
}
