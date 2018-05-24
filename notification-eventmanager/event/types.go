package event

import (
	"fmt"

	"github.com/weaveworks/service/notification-eventmanager/event/configchangedevent"
	"github.com/weaveworks/service/notification-eventmanager/event/deployevent"
	"github.com/weaveworks/service/notification-eventmanager/receiver"
	"github.com/weaveworks/service/notification-eventmanager/types"
	"github.com/weaveworks/service/users/templates"
)

type EventType interface {
	ReceiverData(engine templates.Engine, recv string, e *types.Event) receiver.Data
}
type Types struct {
	engine templates.Engine
}

func NewEventTypes() Types {
	return Types{
		engine: templates.MustNewEngine("../../templates/"),
	}
}

type UnknownEventType struct{}

func (u UnknownEventType) ReceiverData(engine templates.Engine, recv string, e *types.Event) receiver.Data {
	return receiver.UnknownData{Text: fmt.Sprintf("unknown event type: %s", e.Type)}
}

func (t Types) ReceiverData(recv string, e *types.Event) receiver.Data {
	var et EventType

	switch e.Type {
	case "config_changed":
		et = &configchangedevent.Event{}
	case "deploy":
		et = &deployevent.Event{}
	default:
		et = &UnknownEventType{}
	}
	return et.ReceiverData(t.engine, recv, e)
}
