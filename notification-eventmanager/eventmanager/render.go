package eventmanager

import (
	"github.com/weaveworks/service/notification-eventmanager/types"
	"encoding/json"
	"fmt"
)

func renderData(e *types.Event, recv string) json.RawMessage {

	return []byte(fmt.Sprintf("fooz %s", recv))
}
