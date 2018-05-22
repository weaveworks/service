package eventtypes

import (
	"github.com/weaveworks/service/notification-eventmanager/eventtypes/config_changed"
	"github.com/weaveworks/service/notification-eventmanager/types"
)

type OutputRenderer interface {
	Render(recv string, e *types.Event) (types.Output, error)
}

type EventTypes map[string]OutputRenderer

func New() EventTypes {
	ts := EventTypes{}
	ts[config_changed.Type] = config_changed.New()
	return ts
}

func (t EventTypes) Render(recv string, e *types.Event) types.Output {
	res, err := t[e.Type].Render(recv, e)
	if err != nil {
		// TODO: log error
		return nil
	}
	return res
}
