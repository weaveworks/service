package stackdriver

import (
	"github.com/weaveworks/service/notification-eventmanager/types"
)

type ReceiverData struct {
	Text string
}

func (r ReceiverData) Type() string {
	return types.StackdriverReceiver
}
