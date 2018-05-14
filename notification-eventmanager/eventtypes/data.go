package eventtypes

import (
	"encoding/json"
	"github.com/weaveworks/service/notification-eventmanager/eventtypes/config_changed"
)

func Render(eventType string, recv string, data json.RawMessage) (string, error) {
	switch eventType {
	case config_changed.Name:
		d := config_changed.Data{}
		if err := json.Unmarshal(data, &d); err != nil {
			return "", err
		}

	}
	return "", nil
}
