package configchangedevent

import (
	"encoding/json"
	"fmt"

	"github.com/weaveworks/service/notification-eventmanager/receiver"
	"github.com/weaveworks/service/notification-eventmanager/receiver/browser"
	"github.com/weaveworks/service/notification-eventmanager/receiver/email"
	"github.com/weaveworks/service/notification-eventmanager/receiver/slack"
	"github.com/weaveworks/service/notification-eventmanager/receiver/stackdriver"
	"github.com/weaveworks/service/notification-eventmanager/types"
	"github.com/weaveworks/service/users/templates"
)

/*
{
  "browser": {
    "type": "config_changed",
    "text": "The address for \u003cb\u003eslack\u003c/b\u003e was updated!",
    "attachments": null,
    "timestamp": "2018-05-01T22:14:59.160218418Z"
  },
  "email": {
    "subject": "localhost - config_changed",
    "body": "The address for \u003cb\u003eslack\u003c/b\u003e was updated!"
  },
  "slack": {
    "text": "*Instance:* localhost\nThe address for *slack* was updated!"
  },
  "stackdriver": {
    "Timestamp": "2018-05-01T22:14:59.160233461Z",
    "Payload": "The address for \u003cb\u003eslack\u003c/b\u003e was updated!",
    "Labels": {
      "event_type": "config_changed",
      "instance": "localhost"
    }
  }
}

{
  "browser": {
    "type": "config_changed",
    "text": "The event types for \u003cb\u003eslack\u003c/b\u003e were changed: enabled \u003ci\u003e[auto_deploy]\u003c/i\u003e",
    "attachments": null,
    "timestamp": "2018-05-01T22:15:08.357915027Z"
  },
  "email": {
    "subject": "localhost - config_changed",
    "body": "The event types for \u003cb\u003eslack\u003c/b\u003e were changed: enabled \u003ci\u003e[auto_deploy]\u003c/i\u003e"
  },
  "slack": {
    "text": "*Instance:* localhost\nThe event types for *slack* were changed: enabled _[auto_deploy]_"
  },
  "stackdriver": {
    "Timestamp": "2018-05-01T22:15:08.357934425Z",
    "Payload": "The event types for \u003cb\u003eslack\u003c/b\u003e were changed: enabled \u003ci\u003e[auto_deploy]\u003c/i\u003e",
    "Labels": {
      "event_type": "config_changed",
      "instance": "localhost"
    }
  }
}
*/

type Data struct {
	Receiver string
	Address  string
	Enabled  []string
	Disabled []string
}

type Event struct {
}

var tmplExtensions = map[string]string{
	types.BrowserReceiver:     "html",
	types.EmailReceiver:       "html",
	types.SlackReceiver:       "text",
	types.StackdriverReceiver: "text",
}

func (e *Event) ReceiverData(engine templates.Engine, recv string, ev *types.Event) receiver.Data {
	data := Data{}
	if err := json.Unmarshal(ev.Data, &data); err != nil {
		return nil
	}

	tmplname := fmt.Sprintf("%s.%s.%s", ev.Type, recv, tmplExtensions[recv])
	text := e.renderText(engine, tmplname, data)

	switch recv {
	case types.BrowserReceiver:
		return browser.ReceiverData{EventType: ev.Type, Text: text}
	case types.EmailReceiver:
		return email.ReceiverData{
			Subject: fmt.Sprintf("%s – %s", ev.InstanceName, ev.Type),
			Body:    text,
		}
	case types.SlackReceiver:
		return slack.ReceiverData{Text: text}
	case types.StackdriverReceiver:
		return stackdriver.ReceiverData{Text: text}
	}
	return nil
}

func (e *Event) renderText(engine templates.Engine, tmplname string, data Data) string {
	res, err := engine.Bytes(tmplname, data)
	if err != nil {
		// TODO(rndstr): fail better
		res, err = json.Marshal(data)
		if err != nil {
			return "BOOM."
		}
	}
	return string(res)

}
